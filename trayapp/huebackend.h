#ifndef HUEBACKEND_H
#define HUEBACKEND_H

#include <QDBusAbstractInterface>
#include <QDBusConnection>
#include <QDBusMessage>
#include <QDBusPendingCall>
#include <QDBusPendingCallWatcher>
#include <QList>
#include <QMetaObject>
#include <QObject>
#include <QVariant>
#include <functional>
#include <memory>

/**
 * QDBusInterface introspects the service when constructed, a blocking call
 * that waits out the DBus timeout if the backend is hung. The base class
 * generated proxies use makes no call to the service.
 */
class HueBackend : public QDBusAbstractInterface {
  public:
    explicit HueBackend(QObject* parent = nullptr)
        : QDBusAbstractInterface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                                 QDBusConnection::sessionBus(), parent) {}

    /**
     * One timeout covers every call the interface makes, so a call that is
     * allowed to take minutes goes out as a plain message carrying its own.
     */
    QDBusPendingCall asyncCallWithTimeout(const QString& method, int timeoutMs,
                                          const QVariantList& args = {}) {
        QDBusMessage msg = QDBusMessage::createMethodCall(service(), path(), interface(), method);
        msg.setArguments(args);
        return connection().asyncCall(msg, timeoutMs);
    }
};

/**
 * GetScenes makes three sequential bridge requests, each capped at the
 * backend's 10-second HTTP timeout, so a slow bridge pushes the reply past
 * Qt's 25-second default and the scene list never populates.
 */
constexpr int getScenesTimeoutMs = 45 * 1000;

/**
 * Runs done once every call has finished. The watchers are children of
 * context, so done never runs after context is destroyed.
 */
inline void whenFinished(QObject* context, const QList<QDBusPendingCall>& calls,
                         std::function<void()> done) {
    /**
     * Queued, so an empty list still completes after the caller returns, as a
     * non-empty one does.
     */
    if (calls.isEmpty()) {
        QMetaObject::invokeMethod(context, done, Qt::QueuedConnection);
        return;
    }

    auto remaining = std::make_shared<qsizetype>(calls.size());
    for (const QDBusPendingCall& call : calls) {
        auto* watcher = new QDBusPendingCallWatcher(call, context);
        QObject::connect(watcher, &QDBusPendingCallWatcher::finished, context,
                         [remaining, done](QDBusPendingCallWatcher* w) {
                             w->deleteLater();
                             if (--*remaining > 0) {
                                 return;
                             }
                             done();
                         });
    }
}

#endif // HUEBACKEND_H

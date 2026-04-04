#include <QCoreApplication>
#include <QDBusInterface>
#include <QDBusMessage>
#include <QDBusReply>
#include <QDebug>
#include <QVariantMap>

int main(int argc, char* argv[]) {
    QCoreApplication app(argc, argv);

    QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                         QDBusConnection::sessionBus());

    qDebug() << "Testing GetSyncSettings...";
    QDBusMessage reply = iface.call("GetSyncSettings");

    if (reply.type() == QDBusMessage::ErrorMessage) {
        qDebug() << "Error:" << reply.errorMessage();
        return 1;
    }

    qDebug() << "Reply arguments:" << reply.arguments();

    // Try to extract as QVariantMap
    if (reply.arguments().count() > 0) {
        QVariant arg = reply.arguments().at(0);
        qDebug() << "Argument type:" << arg.typeName();

        // Convert DBusArgument to QVariantMap
        QVariantMap map = qdbus_cast<QVariantMap>(arg);
        qDebug() << "Map contents:" << map;
    }

    return 0;
}

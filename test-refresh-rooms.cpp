#include <QCoreApplication>
#include <QDBusInterface>
#include <QDBusReply>
#include <QDBusArgument>
#include <QDBusMessage>
#include <QDebug>

int main(int argc, char *argv[]) {
    QCoreApplication app(argc, argv);
    
    qDebug() << "Testing GetGroupedLights extraction...";
    
    QDBusInterface iface("org.kde.plasma.hue", 
                         "/org/kde/plasma/hue",
                         "org.kde.plasma.hue", 
                         QDBusConnection::sessionBus());
    
    if (!iface.isValid()) {
        qDebug() << "❌ DBus interface invalid:" << iface.lastError().message();
        return 1;
    }
    
    qDebug() << "✅ DBus interface valid";
    
    // Use QDBusMessage directly to get mutable arguments
    QDBusMessage reply = iface.call("GetGroupedLights");
    
    if (reply.type() == QDBusMessage::ErrorMessage) {
        qDebug() << "❌ Reply error:" << reply.errorMessage();
        return 1;
    }
    
    if (reply.arguments().isEmpty()) {
        qDebug() << "❌ No arguments in reply";
        return 1;
    }
    
    qDebug() << "✅ Reply valid";
    
    // Get the first argument as a QDBusArgument - this creates a mutable copy
    qDebug() << "Attempting to extract QDBusArgument from message arguments...";
    
    QVariant var = reply.arguments().at(0);
    if (!var.canConvert<QDBusArgument>()) {
        qDebug() << "❌ Cannot convert to QDBusArgument";
        return 1;
    }
    
    // Extract to get a mutable copy
    const QDBusArgument arg = var.value<QDBusArgument>();
    
    qDebug() << "Calling beginArray()...";
    arg.beginArray();
    
    int count = 0;
    while (!arg.atEnd()) {
        qDebug() << "Processing struct" << count;
        arg.beginStructure();
        QString id, name, type;
        arg >> id >> name >> type;
        arg.endStructure();
        
        qDebug() << "  ID:" << id;
        qDebug() << "  Name:" << name;
        qDebug() << "  Type:" << type;
        
        count++;
    }
    arg.endArray();
    
    qDebug() << "✅ Successfully extracted" << count << "rooms/zones";
    
    return 0;
}

#include <QApplication>
#include <QMenu>
#include <QAction>
#include <QDBusInterface>
#include <QDBusReply>
#include <QDBusArgument>
#include <QMessageBox>
#include <QInputDialog>
#include <QMap>
#include <QSlider>
#include <QVBoxLayout>
#include <QHBoxLayout>
#include <QWidget>
#include <QLabel>
#include <QPushButton>
#include <QCheckBox>
#include <QListWidget>
#include <QDialog>
#include <QDebug>
#include <QIcon>
#include <QTimer>
#include <QProcess>
#include <KStatusNotifierItem>

class HueControlDialog : public QDialog {
    Q_OBJECT

public:
    HueControlDialog(QWidget *parent = nullptr) : QDialog(parent) {
        setWindowTitle("Hue Control");
        setMinimumWidth(400);
        
        auto layout = new QVBoxLayout(this);
        
        // Status label
        statusLabel = new QLabel("Connecting to Hue service...", this);
        layout->addWidget(statusLabel);
        
        // Power control
        auto powerLayout = new QHBoxLayout();
        powerCheckbox = new QCheckBox("Power", this);
        powerLayout->addWidget(powerCheckbox);
        powerLayout->addStretch();
        layout->addLayout(powerLayout);
        
        connect(powerCheckbox, &QCheckBox::toggled, this, &HueControlDialog::onPowerToggled);
        
        // Brightness control
        auto brightnessLayout = new QVBoxLayout();
        brightnessLayout->addWidget(new QLabel("Brightness:", this));
        brightnessSlider = new QSlider(Qt::Horizontal, this);
        brightnessSlider->setRange(0, 100);
        brightnessSlider->setValue(100);
        brightnessLayout->addWidget(brightnessSlider);
        brightnessValueLabel = new QLabel("100%", this);
        brightnessLayout->addWidget(brightnessValueLabel);
        layout->addLayout(brightnessLayout);
        
        connect(brightnessSlider, &QSlider::valueChanged, this, &HueControlDialog::onBrightnessChanged);
        
        // Scene list
        layout->addWidget(new QLabel("Scenes:", this));
        sceneList = new QListWidget(this);
        layout->addWidget(sceneList);
        
        connect(sceneList, &QListWidget::itemDoubleClicked, this, &HueControlDialog::onSceneActivated);
        
        // Sync control
        auto syncLayout = new QHBoxLayout();
        syncButton = new QPushButton("Screen Sync (Coming soon)", this);
        syncButton->setEnabled(false);
        syncLayout->addWidget(syncButton);
        syncStatusLabel = new QLabel("Feature not yet implemented", this);
        syncLayout->addWidget(syncStatusLabel);
        syncLayout->addStretch();
        layout->addLayout(syncLayout);
        
        connect(syncButton, &QPushButton::clicked, this, &HueControlDialog::onSyncToggled);
        
        // Settings button
        auto settingsBtn = new QPushButton("Select Room/Zone", this);
        connect(settingsBtn, &QPushButton::clicked, this, &HueControlDialog::onSettingsClicked);
        layout->addWidget(settingsBtn);
        
        // Refresh button
        auto refreshBtn = new QPushButton("Refresh", this);
        layout->addWidget(refreshBtn);
        connect(refreshBtn, &QPushButton::clicked, this, &HueControlDialog::refresh);
        
        // Initial load
        refresh();
    }

public slots:
    void refresh() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", 
                           "org.kde.plasma.hue", QDBusConnection::sessionBus());
        
        if (!iface.isValid()) {
            statusLabel->setText("❌ DBus service not available");
            return;
        }
        
        // Get status
        QDBusReply<QString> statusReply = iface.call("GetStatus");
        if (statusReply.isValid()) {
            statusLabel->setText("✅ " + statusReply.value());
        }
        
        // Get current state (power and brightness)
        QDBusMessage stateMsg = iface.call("GetState");
        if (stateMsg.type() == QDBusMessage::ReplyMessage && stateMsg.arguments().size() >= 3) {
            bool power = stateMsg.arguments().at(0).toBool();
            int brightness = stateMsg.arguments().at(1).toInt();
            bool success = stateMsg.arguments().at(2).toBool();
            
            if (success) {
                // Cancel any pending brightness changes since we're syncing with actual state
                if (brightnessTimer && brightnessTimer->isActive()) {
                    brightnessTimer->stop();
                }
                
                // Block signals while updating to avoid triggering DBus calls
                powerCheckbox->blockSignals(true);
                brightnessSlider->blockSignals(true);
                
                // Always update to actual state from lights
                powerCheckbox->setChecked(power);
                brightnessSlider->setValue(brightness);
                brightnessValueLabel->setText(QString::number(brightness) + "%");
                
                // Update pendingBrightness to match actual state
                pendingBrightness = brightness;
                
                powerCheckbox->blockSignals(false);
                brightnessSlider->blockSignals(false);
            }
        }
        
        // Get scenes
        QDBusReply<QStringList> scenesReply = iface.call("GetScenes");
        if (scenesReply.isValid()) {
            sceneList->clear();
            sceneList->addItems(scenesReply.value());
        }
        
        // Get sync status
        QDBusReply<bool> syncReply = iface.call("IsSyncing");
        if (syncReply.isValid()) {
            bool syncing = syncReply.value();
            syncButton->setText(syncing ? "Stop Sync" : "Start Sync");
            syncStatusLabel->setText(syncing ? "✅ Syncing" : "Not syncing");
        }
    }

private slots:
    void onPowerToggled(bool checked) {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", 
                           "org.kde.plasma.hue", QDBusConnection::sessionBus());
        iface.call("SetPower", checked);
        
        // Refresh state after a short delay to get updated brightness
        QTimer::singleShot(500, this, &HueControlDialog::refresh);
    }
    
    void onBrightnessChanged(int value) {
        brightnessValueLabel->setText(QString::number(value) + "%");
        
        // Store the value and restart the timer
        pendingBrightness = value;
        if (!brightnessTimer) {
            brightnessTimer = new QTimer(this);
            brightnessTimer->setSingleShot(true);
            connect(brightnessTimer, &QTimer::timeout, this, &HueControlDialog::applyBrightness);
        }
        brightnessTimer->start(300); // Wait 300ms after user stops dragging
    }
    
    void applyBrightness() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", 
                           "org.kde.plasma.hue", QDBusConnection::sessionBus());
        iface.call("SetBrightness", pendingBrightness);
    }
    
    void onSceneActivated(QListWidgetItem *item) {
        QString sceneName = item->text();
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", 
                           "org.kde.plasma.hue", QDBusConnection::sessionBus());
        
        if (!iface.isValid()) {
            statusLabel->setText("❌ Backend not available");
            QMessageBox::warning(this, "Hue Control", "Backend service is not running");
            return;
        }
        
        QDBusReply<QString> reply = iface.call("ActivateScene", sceneName);
        if (reply.isValid()) {
            QString result = reply.value();
            statusLabel->setText("✅ " + result);
            
            // Show success notification
            QProcess::startDetached("notify-send", QStringList() 
                << "Hue Scene" 
                << "Activated: " + sceneName
                << "--icon=preferences-desktop-display-color"
                << "--urgency=low");
        } else {
            QString error = reply.error().message();
            statusLabel->setText("❌ Failed: " + error);
            
            // Show error notification
            QProcess::startDetached("notify-send", QStringList() 
                << "Hue Scene Error" 
                << "Failed to activate scene: " + sceneName
                << "--icon=dialog-error"
                << "--urgency=normal");
        }
    }
    
    void onSyncToggled() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", 
                           "org.kde.plasma.hue", QDBusConnection::sessionBus());
        
        QDBusReply<bool> syncReply = iface.call("IsSyncing");
        bool currentlySyncing = syncReply.isValid() ? syncReply.value() : false;
        
        if (currentlySyncing) {
            iface.call("StopSync");
        } else {
            iface.call("StartSync");
        }
        
        QTimer::singleShot(500, this, &HueControlDialog::refresh);
    }
    
    void onSettingsClicked() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", 
                           "org.kde.plasma.hue", QDBusConnection::sessionBus());
        
        // Get available grouped lights
        QDBusReply<QDBusArgument> reply = iface.call("GetGroupedLights");
        if (!reply.isValid()) {
            QMessageBox::warning(this, "Error", "Failed to get grouped lights: " + reply.error().message());
            return;
        }
        
        // Parse the array of structs
        QStringList items;
        QMap<QString, QString> idMap; // Display name -> ID
        
        QDBusArgument arg = reply.value();
        arg.beginArray();
        while (!arg.atEnd()) {
            arg.beginStructure();
            QString id, name, type;
            arg >> id >> name >> type;
            arg.endStructure();
            
            QString displayName = name + " (" + type + ")";
            items << displayName;
            idMap[displayName] = id;
        }
        arg.endArray();
        
        if (items.isEmpty()) {
            QMessageBox::information(this, "No Lights", "No grouped lights (rooms/zones) found.");
            return;
        }
        
        // Show selection dialog
        bool ok;
        QString selected = QInputDialog::getItem(this, "Select Room/Zone",
                                                 "Choose a room or zone to control:",
                                                 items, 0, false, &ok);
        
        if (ok && !selected.isEmpty()) {
            QString selectedID = idMap[selected];
            
            // Call backend to update config
            QDBusReply<bool> setReply = iface.call("SetGroupedLight", selectedID);
            
            if (setReply.isValid() && setReply.value()) {
                QMessageBox::information(this, "Success", 
                    "Grouped light set to: " + selected + "\n\n"
                    "Configuration saved successfully!");
                    
                // Refresh to update UI with new light
                refresh();
            } else {
                QString errorMsg = setReply.isValid() ? 
                    "Failed to save configuration" : 
                    setReply.error().message();
                QMessageBox::warning(this, "Error", "Failed to update config: " + errorMsg);
            }
        }
    }

private:
    QLabel *statusLabel;
    QCheckBox *powerCheckbox;
    QSlider *brightnessSlider;
    QLabel *brightnessValueLabel;
    QListWidget *sceneList;
    QPushButton *syncButton;
    QLabel *syncStatusLabel;
    QTimer *brightnessTimer = nullptr;
    int pendingBrightness = 100;
};

class HueTrayApp : public QApplication {
    Q_OBJECT

public:
    HueTrayApp(int &argc, char **argv) : QApplication(argc, argv) {
        setQuitOnLastWindowClosed(false);
        
        // Create KDE StatusNotifierItem (native Plasma system tray)
        sni = new KStatusNotifierItem(this);
        sni->setIconByName("preferences-desktop-display-color");
        sni->setTitle("Hue Control");
        sni->setToolTip("preferences-desktop-display-color", "Hue Control", "Control Philips Hue lights");
        sni->setCategory(KStatusNotifierItem::Hardware);
        sni->setStatus(KStatusNotifierItem::Active);
        
        // Create menu
        auto menu = new QMenu();
        
        auto showAction = menu->addAction("Show Control Panel");
        connect(showAction, &QAction::triggered, this, &HueTrayApp::showControlDialog);
        
        sni->setContextMenu(menu);
        sni->setStandardActionsEnabled(true); // KStatusNotifierItem adds Quit automatically
        
        // Show control dialog on activation
        connect(sni, &KStatusNotifierItem::activateRequested, this, &HueTrayApp::showControlDialog);
        
        // Create control dialog
        controlDialog = new HueControlDialog();
        
        qDebug() << "KStatusNotifierItem created and activated";
    }

private slots:
    void showControlDialog() {
        if (controlDialog) {
            controlDialog->show();
            controlDialog->raise();
            controlDialog->activateWindow();
            controlDialog->refresh();
        }
    }

private:
    KStatusNotifierItem *sni;
    HueControlDialog *controlDialog;
};

int main(int argc, char *argv[]) {
    HueTrayApp app(argc, argv);
    qDebug() << "Hue Control tray app starting...";
    return app.exec();
}

#include "main.moc"

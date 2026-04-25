#include "settingsdialog.h"
#include <KNotification>
#include <KStatusNotifierItem>
#include <QAction>
#include <QApplication>
#include <QCheckBox>
#include <QDBusArgument>
#include <QDBusInterface>
#include <QDBusMessage>
#include <QDBusPendingCall>
#include <QDBusPendingReply>
#include <QDBusReply>
#include <QDebug>
#include <QDialog>
#include <QHBoxLayout>
#include <QIcon>
#include <QInputDialog>
#include <QLabel>
#include <QListWidget>
#include <QMap>
#include <QMenu>
#include <QMessageBox>
#include <QProcess>
#include <QPushButton>
#include <QSlider>
#include <QTimer>
#include <QVBoxLayout>
#include <QWidget>

class HueControlDialog : public QDialog {
    Q_OBJECT

  public:
    HueControlDialog(QWidget* parent = nullptr) : QDialog(parent) {
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

        connect(brightnessSlider, &QSlider::valueChanged, this,
                &HueControlDialog::onBrightnessChanged);

        // Scene list
        layout->addWidget(new QLabel("Scenes:", this));
        sceneList = new QListWidget(this);
        layout->addWidget(sceneList);

        connect(sceneList, &QListWidget::itemDoubleClicked, this,
                &HueControlDialog::onSceneActivated);

        // Sync control
        auto syncLayout = new QHBoxLayout();
        syncButton = new QPushButton("Start Screen Sync", this);
        syncButton->setEnabled(true); // Enable sync button - backend ready!
        syncLayout->addWidget(syncButton);
        syncStatusLabel = new QLabel("Not syncing", this);
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

        // Check connection status periodically
        QTimer* connectionTimer = new QTimer(this);
        connect(connectionTimer, &QTimer::timeout, this, &HueControlDialog::checkConnectionStatus);
        connectionTimer->start(10000); // Check every 10 seconds

        // Check gaming mode status periodically
        QTimer* gamingTimer = new QTimer(this);
        connect(gamingTimer, &QTimer::timeout, this, &HueControlDialog::updateGamingStatus);
        gamingTimer->start(2000); // Check every 2 seconds
    }

  public slots:
    void refresh() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            statusLabel->setText("❌ DBus service not available");
            showErrorNotification("Service Unavailable", "KDE Hue Control backend is not running.\n"
                                                         "Try: systemctl --user start hue-backend");
            return;
        }

        // Check connection status first
        checkConnectionStatus();

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
            syncButton->setText(syncing ? "Stop Screen Sync" : "Start Screen Sync");
            updateGamingStatus();
        }
    }

  private slots:
    void onPowerToggled(bool checked) {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());
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
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());
        iface.call("SetBrightness", pendingBrightness);
    }

    void onSceneActivated(QListWidgetItem* item) {
        QString sceneName = item->text();
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            statusLabel->setText("❌ Backend not available");
            showErrorNotification("Service Unavailable", "Backend service is not running");
            return;
        }

        // Show loading state
        statusLabel->setText("⏳ Activating scene...");
        sceneList->setEnabled(false);

        // Make async call to avoid blocking UI
        QDBusPendingCall call = iface.asyncCall("ActivateScene", sceneName);
        QDBusPendingCallWatcher* watcher = new QDBusPendingCallWatcher(call, this);

        connect(watcher, &QDBusPendingCallWatcher::finished, this,
                [this, sceneName, item](QDBusPendingCallWatcher* w) {
                    sceneList->setEnabled(true);
                    QDBusPendingReply<QString> reply = *w;

                    if (reply.isError()) {
                        QString error = reply.error().message();
                        statusLabel->setText("❌ Failed: " + error);

                        // Check if it's a connection error
                        if (error.contains("unreachable") || error.contains("timeout")) {
                            showErrorNotification(
                                "Bridge Unreachable",
                                "Cannot connect to Hue Bridge. Check your network connection.");
                        } else {
                            showErrorNotification("Scene Activation Failed",
                                                  "Failed to activate scene: " + sceneName +
                                                      "\n\n" + error);
                        }
                    } else {
                        QString result = reply.value();
                        statusLabel->setText("✅ " + result);

                        // Show success notification
                        KNotification* notif = new KNotification("sceneActivated");
                        notif->setTitle("Scene Activated");
                        notif->setText(sceneName);
                        notif->setIconName("preferences-desktop-display-color");
                        notif->sendEvent();
                        // Auto-delete notification after sending to prevent memory leak
                        notif->deleteLater();
                    }

                    w->deleteLater();
                });
    }

    void onSyncToggled() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        QDBusReply<bool> syncReply = iface.call("IsSyncing");
        bool currentlySyncing = syncReply.isValid() ? syncReply.value() : false;

        if (currentlySyncing) {
            // Stopping sync
            syncButton->setEnabled(false);
            syncButton->setText("⏳ Stopping...");

            QDBusReply<bool> reply = iface.call("StopSync");
            if (reply.isValid() && reply.value()) {
                syncButton->setText("Start Screen Sync");
                syncStatusLabel->setText("Not syncing");

                KNotification* notif = new KNotification("syncStopped");
                notif->setTitle("Screen Sync Stopped");
                notif->setText("Lights are no longer syncing with screen");
                notif->setIconName("dialog-information");
                notif->sendEvent();
                // Auto-delete notification after sending to prevent memory leak
                notif->deleteLater();
            } else {
                syncButton->setText("Stop Screen Sync");
                showErrorNotification("Failed to Stop Sync", reply.error().message());
            }
            syncButton->setEnabled(true);
        } else {
            // Starting sync
            syncButton->setEnabled(false);
            syncButton->setText("⏳ Starting sync...");
            syncStatusLabel->setText("Waiting for permission...");

            QDBusPendingCall call = iface.asyncCall("StartSync");
            QDBusPendingCallWatcher* watcher = new QDBusPendingCallWatcher(call, this);

            connect(watcher, &QDBusPendingCallWatcher::finished, this,
                    [this](QDBusPendingCallWatcher* w) {
                        syncButton->setEnabled(true);
                        QDBusPendingReply<bool> reply = *w;

                        if (reply.isError() || !reply.value()) {
                            QString error = reply.error().message();
                            syncButton->setText("Start Screen Sync");
                            syncStatusLabel->setText("Not syncing");

                            // Parse portal errors for user-friendly messages
                            if (error.contains("PortalError:permission_denied")) {
                                QString hint = error.section(':', 2);

                                KNotification* notif = new KNotification("syncFailed");
                                notif->setTitle("Screen Sharing Permission Denied");
                                notif->setText(
                                    "Please approve the screen sharing dialog when prompted.\n\n" +
                                    hint + "\n\nClick 'Start Screen Sync' to try again.");
                                notif->setIconName("dialog-warning");
                                notif->sendEvent();
                                // Auto-delete notification after sending to prevent memory leak
                                notif->deleteLater();
                            } else if (error.contains("PortalError:")) {
                                QString errorType = error.section(':', 1, 1);
                                QString hint = error.section(':', 2);

                                showErrorNotification("Screen Sync Failed",
                                                      "Error: " + errorType + "\n\n" + hint);
                            } else if (error.contains("sync engine not available")) {
                                showErrorNotification("Screen Sync Not Configured",
                                                      "Entertainment API is not configured.\n\n"
                                                      "Please set up Entertainment Area in Hue app "
                                                      "and configure clientkey.");
                            } else {
                                showErrorNotification("Failed to Start Screen Sync", error);
                            }
                        } else {
                            syncButton->setText("Stop Screen Sync");
                            syncStatusLabel->setText("✅ Syncing");

                            KNotification* notif = new KNotification("syncStarted");
                            notif->setTitle("Screen Sync Started");
                            notif->setText("Lights are now syncing with your screen at 30 FPS");
                            notif->setIconName("preferences-desktop-display");
                            notif->sendEvent();
                        }

                        w->deleteLater();
                    });
        }
    }

    void onSettingsClicked() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        // Get available grouped lights
        QDBusMessage reply = iface.call("GetGroupedLights");
        if (reply.type() == QDBusMessage::ErrorMessage) {
            QMessageBox::warning(this, "Error",
                                 "Failed to get grouped lights: " + reply.errorMessage());
            return;
        }

        if (reply.arguments().isEmpty()) {
            QMessageBox::warning(this, "Error", "No data received from backend");
            return;
        }

        // Parse the array of structs - MUST use const QDBusArgument!
        QStringList items;
        QMap<QString, QString> idMap; // Display name -> ID

        QVariant var = reply.arguments().at(0);
        if (!var.canConvert<QDBusArgument>()) {
            QMessageBox::warning(this, "Error", "Invalid data format from backend");
            return;
        }

        const QDBusArgument arg = var.value<QDBusArgument>();
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
        QString selected = QInputDialog::getItem(
            this, "Select Room/Zone", "Choose a room or zone to control:", items, 0, false, &ok);

        if (ok && !selected.isEmpty()) {
            QString selectedID = idMap[selected];

            // Call backend to update config
            QDBusReply<bool> setReply = iface.call("SetGroupedLight", selectedID);

            if (setReply.isValid() && setReply.value()) {
                QMessageBox::information(this, "Success",
                                         "Grouped light set to: " + selected +
                                             "\n\n"
                                             "Configuration saved successfully!");

                // Refresh to update UI with new light
                refresh();
            } else {
                QString errorMsg = setReply.isValid() ? "Failed to save configuration"
                                                      : setReply.error().message();
                QMessageBox::warning(this, "Error", "Failed to update config: " + errorMsg);
            }
        }
    }

    void checkConnectionStatus() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            return; // Service not running
        }

        QDBusReply<QVariantMap> reply = iface.call("GetConnectionStatus");
        if (!reply.isValid()) {
            return; // Method not available (old backend)
        }

        QVariantMap status = reply.value();
        bool connected = status["connected"].toBool();
        QString lastError = status["lastError"].toString();
        QString bridgeIP = status["bridgeIP"].toString();

        if (!connected && !lastError.isEmpty()) {
            // Bridge is unreachable - show notification once
            static bool errorShown = false;
            if (!errorShown) {
                errorShown = true;

                KNotification* notif = new KNotification("connectionFailed");
                notif->setTitle("Hue Bridge Unreachable");
                notif->setText(QString("Cannot connect to bridge at %1\n\n%2\n\n"
                                       "Open the control panel and click 'Retry' to reconnect.")
                                   .arg(bridgeIP)
                                   .arg(lastError));
                notif->setIconName("network-disconnect");
                notif->sendEvent();

                // Reset flag after 30 seconds to allow showing again
                QTimer::singleShot(30000, []() {
                    static bool errorShown = false;
                    errorShown = false;
                });
            }

            statusLabel->setText("❌ Bridge unreachable");
        } else if (connected) {
            // Connection restored
            static bool wasDisconnected = false;
            if (wasDisconnected) {
                wasDisconnected = false;
                statusLabel->setText("✅ Connection restored");

                KNotification* notif = new KNotification("connectionRestored");
                notif->setTitle("Bridge Connection Restored");
                notif->setText("Successfully reconnected to Hue Bridge");
                notif->setIconName("network-connect");
                notif->sendEvent();

                refresh(); // Reload scenes and state
            }
        }
    }

    void retryConnection() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        statusLabel->setText("🔄 Retrying connection...");

        QDBusReply<bool> reply = iface.call("RetryConnection");
        if (reply.isValid() && reply.value()) {
            statusLabel->setText("✅ Connection restored");

            KNotification* notif = new KNotification("connectionRestored");
            notif->setTitle("Connection Restored");
            notif->setText("Successfully reconnected to Hue Bridge");
            notif->setIconName("network-connect");
            notif->sendEvent();

            refresh();
        } else {
            statusLabel->setText("❌ Still unreachable");
            showErrorNotification("Retry Failed",
                                  "Bridge is still unreachable. Check your network connection.");
        }
    }

    void updateGamingStatus() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            return;
        }

        // Check if gaming mode is enabled in config
        QDBusReply<bool> enabledReply = iface.call("IsGamingModeEnabled");
        bool gamingEnabled = enabledReply.isValid() && enabledReply.value();

        // Check if gaming is currently active (game detected)
        QDBusReply<bool> activeReply = iface.call("IsGamingModeActive");
        bool gamingActive = activeReply.isValid() && activeReply.value();

        // Check if syncing
        QDBusReply<bool> syncReply = iface.call("IsSyncing");
        bool syncing = syncReply.isValid() && syncReply.value();

        // Update sync status label with gaming mode info
        if (syncing && gamingActive) {
            syncStatusLabel->setText("✅ Syncing (🎮 Gaming Mode)");
        } else if (syncing) {
            syncStatusLabel->setText("✅ Syncing");
        } else if (gamingEnabled && gamingActive) {
            syncStatusLabel->setText("🎮 Gaming detected - waiting to sync...");
        } else if (gamingEnabled) {
            syncStatusLabel->setText("Not syncing (Gaming Mode: armed)");
        } else {
            syncStatusLabel->setText("Not syncing");
        }
    }

    void showErrorNotification(const QString& title, const QString& message) {
        KNotification* notif = new KNotification("error");
        notif->setTitle(title);
        notif->setText(message);
        notif->setIconName("dialog-error");
        notif->sendEvent();
    }

  private:
    QLabel* statusLabel;
    QCheckBox* powerCheckbox;
    QSlider* brightnessSlider;
    QLabel* brightnessValueLabel;
    QListWidget* sceneList;
    QPushButton* syncButton;
    QLabel* syncStatusLabel;
    QTimer* brightnessTimer = nullptr;
    int pendingBrightness = 100;
};

class HueTrayApp : public QApplication {
    Q_OBJECT

  public:
    HueTrayApp(int& argc, char** argv) : QApplication(argc, argv) {
        setQuitOnLastWindowClosed(false);

        // Initialize default icon names
        gamingIconName = "applications-games";
        syncingIconName = "media-record";
        idleIconName = "preferences-desktop-display-color";

        // Load icon names from backend config
        loadIconNames();

        // Create KDE StatusNotifierItem (native Plasma system tray)
        sni = new KStatusNotifierItem(this);
        sni->setIconByName(idleIconName);
        sni->setTitle("Hue Control");
        sni->setToolTip(idleIconName, "Hue Control",
                        "Control Philips Hue lights");
        sni->setCategory(KStatusNotifierItem::Hardware);
        sni->setStatus(KStatusNotifierItem::Active);

        // Create menu
        auto menu = new QMenu();

        auto showAction = menu->addAction(QIcon::fromTheme("view-form"), "Show Control Panel");
        connect(showAction, &QAction::triggered, this, &HueTrayApp::showControlDialog);

        menu->addSeparator();

        auto settingsAction = menu->addAction(QIcon::fromTheme("configure"), "Settings...");
        connect(settingsAction, &QAction::triggered, this, &HueTrayApp::showSettingsDialog);

        sni->setContextMenu(menu);
        sni->setStandardActionsEnabled(true); // KStatusNotifierItem adds Quit automatically

        // Show control dialog on activation
        connect(sni, &KStatusNotifierItem::activateRequested, this, &HueTrayApp::showControlDialog);

        // Create control dialog
        controlDialog = new HueControlDialog();

        // Update tooltip periodically with gaming mode status
        QTimer* tooltipTimer = new QTimer(this);
        connect(tooltipTimer, &QTimer::timeout, this, &HueTrayApp::updateTooltip);
        tooltipTimer->start(3000); // Update every 3 seconds
        updateTooltip(); // Initial update
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

    void showSettingsDialog() {
        try {
            SettingsDialog* dialog = new SettingsDialog();
            dialog->exec();
            delete dialog;
            // Reload icon names after settings dialog closes (user may have changed them)
            loadIconNames();
            updateTooltip(); // Update icon immediately
        } catch (...) {
            QMessageBox::critical(nullptr, "Error", "Failed to create settings dialog");
        }
    }

    void loadIconNames() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            // Backend not running, keep defaults
            return;
        }

        QDBusReply<QDBusVariant> reply = iface.call("GetTrayIcons");
        if (!reply.isValid()) {
            // Method failed, keep defaults
            return;
        }

        // GetTrayIcons returns (string gaming, string syncing, string idle)
        // DBus packs multiple return values into a struct
        const QDBusArgument arg = reply.value().variant().value<QDBusArgument>();
        arg.beginStructure();
        arg >> gamingIconName >> syncingIconName >> idleIconName;
        arg.endStructure();
    }

    void updateTooltip() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            sni->setToolTip(idleIconName, "Hue Control",
                            "Backend not running");
            sni->setIconByName(idleIconName);
            return;
        }

        // Get gaming mode status
        QDBusReply<bool> enabledReply = iface.call("IsGamingModeEnabled");
        bool gamingEnabled = enabledReply.isValid() && enabledReply.value();

        QDBusReply<bool> activeReply = iface.call("IsGamingModeActive");
        bool gamingActive = activeReply.isValid() && activeReply.value();

        QDBusReply<bool> syncReply = iface.call("IsSyncing");
        bool syncing = syncReply.isValid() && syncReply.value();

        // Update icon based on gaming + sync state
        if (syncing && gamingActive) {
            sni->setIconByName(gamingIconName); // Gaming icon when gaming + syncing
        } else if (syncing) {
            sni->setIconByName(syncingIconName); // Sync active icon (non-gaming)
        } else {
            sni->setIconByName(idleIconName); // Default icon (idle)
        }

        // Build tooltip text
        QString tooltipText = "Control Philips Hue lights";
        
        if (syncing && gamingActive) {
            tooltipText = "🎮 Gaming Mode Active - Syncing to screen";
        } else if (syncing) {
            tooltipText = "Syncing lights to screen";
        } else if (gamingEnabled && gamingActive) {
            tooltipText = "🎮 Game detected - preparing to sync...";
        } else if (gamingEnabled) {
            tooltipText = "Gaming Mode enabled (armed)";
        }

        sni->setToolTip(idleIconName, "Hue Control", tooltipText);
    }

  private:
    KStatusNotifierItem* sni;
    HueControlDialog* controlDialog;
    
    // Cached icon names from backend config
    QString gamingIconName;
    QString syncingIconName;
    QString idleIconName;
};

int main(int argc, char* argv[]) {
    HueTrayApp app(argc, argv);
    qDebug() << "Hue Control tray app starting...";
    return app.exec();
}

#include "main.moc"

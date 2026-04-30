#include "settingsdialog.h"
#include <KNotification>
#include <KStatusNotifierItem>
#include <QAction>
#include <QApplication>
#include <QCheckBox>
#include <QDateTime>
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
#include <QSet>
#include <QSlider>
#include <QTimer>
#include <QVBoxLayout>
#include <QWidget>

// Enum for connection state
enum ConnectionState {
    CONNECTING,
    CONNECTED,
    DISCONNECTED,
    ERROR
};

class HueControlDialog : public QDialog {
    Q_OBJECT

  public:
    HueControlDialog(QWidget* parent = nullptr) : QDialog(parent) {
        setWindowTitle("Hue Control");
        setMinimumWidth(450);

        auto layout = new QVBoxLayout(this);

        // Status section
        auto statusLayout = new QHBoxLayout();
        auto statusPrefixLabel = new QLabel("Status:", this);
        statusPrefixLabel->setStyleSheet("QLabel { font-weight: bold; }");
        statusLayout->addWidget(statusPrefixLabel);
        statusLabel = new QLabel("Connecting to Hue service...", this);
        statusLabel->setWordWrap(true);
        statusLayout->addWidget(statusLabel, 1);
        layout->addLayout(statusLayout);

        // Connection status details (collapsible)
        connectionDetailsLabel = new QLabel(this);
        connectionDetailsLabel->setWordWrap(true);
        connectionDetailsLabel->setStyleSheet("QLabel { color: gray; font-size: 10pt; padding: 5px; }");
        connectionDetailsLabel->hide();
        layout->addWidget(connectionDetailsLabel);

        layout->addSpacing(10);

        // Power control with icon
        auto powerLayout = new QHBoxLayout();
        auto powerIcon = new QLabel(this);
        powerIcon->setPixmap(QIcon::fromTheme("system-shutdown").pixmap(16, 16));
        powerLayout->addWidget(powerIcon);
        powerCheckbox = new QCheckBox("Power", this);
        powerLayout->addWidget(powerCheckbox);
        powerLayout->addStretch();
        layout->addLayout(powerLayout);

        connect(powerCheckbox, &QCheckBox::toggled, this, &HueControlDialog::onPowerToggled);

        // Brightness control with preset buttons
        auto brightnessLayout = new QVBoxLayout();
        auto brightnessHeaderLayout = new QHBoxLayout();
        auto brightnessIcon = new QLabel(this);
        brightnessIcon->setPixmap(QIcon::fromTheme("brightness-high").pixmap(16, 16));
        brightnessHeaderLayout->addWidget(brightnessIcon);
        brightnessHeaderLayout->addWidget(new QLabel("Brightness:", this));
        brightnessHeaderLayout->addStretch();
        brightnessValueLabel = new QLabel("100%", this);
        brightnessValueLabel->setStyleSheet("QLabel { font-weight: bold; }");
        brightnessHeaderLayout->addWidget(brightnessValueLabel);
        brightnessLayout->addLayout(brightnessHeaderLayout);

        brightnessSlider = new QSlider(Qt::Horizontal, this);
        brightnessSlider->setRange(0, 100);
        brightnessSlider->setValue(100);
        brightnessLayout->addWidget(brightnessSlider);

        // Preset brightness buttons
        auto presetsLayout = new QHBoxLayout();
        presetsLayout->addWidget(new QLabel("Quick:", this));
        preset25Button = new QPushButton("25%", this);
        preset50Button = new QPushButton("50%", this);
        preset75Button = new QPushButton("75%", this);
        preset100Button = new QPushButton("100%", this);
        preset25Button->setMaximumWidth(60);
        preset50Button->setMaximumWidth(60);
        preset75Button->setMaximumWidth(60);
        preset100Button->setMaximumWidth(60);
        preset25Button->setCheckable(true);
        preset50Button->setCheckable(true);
        preset75Button->setCheckable(true);
        preset100Button->setCheckable(true);
        presetsLayout->addWidget(preset25Button);
        presetsLayout->addWidget(preset50Button);
        presetsLayout->addWidget(preset75Button);
        presetsLayout->addWidget(preset100Button);
        presetsLayout->addStretch();
        brightnessLayout->addLayout(presetsLayout);

        layout->addLayout(brightnessLayout);

        connect(brightnessSlider, &QSlider::valueChanged, this,
                &HueControlDialog::onBrightnessChanged);
        connect(preset25Button, &QPushButton::clicked, [this]() { brightnessSlider->setValue(25); });
        connect(preset50Button, &QPushButton::clicked, [this]() { brightnessSlider->setValue(50); });
        connect(preset75Button, &QPushButton::clicked, [this]() { brightnessSlider->setValue(75); });
        connect(preset100Button, &QPushButton::clicked, [this]() { brightnessSlider->setValue(100); });

        layout->addSpacing(10);

        // Scene list with current scene indicator
        auto sceneHeaderLayout = new QHBoxLayout();
        auto sceneIcon = new QLabel(this);
        sceneIcon->setPixmap(QIcon::fromTheme("preferences-desktop-display-color").pixmap(16, 16));
        sceneHeaderLayout->addWidget(sceneIcon);
        sceneHeaderLayout->addWidget(new QLabel("Scenes:", this));
        sceneCountLabel = new QLabel("(0)", this);
        sceneCountLabel->setStyleSheet("QLabel { color: gray; }");
        sceneHeaderLayout->addWidget(sceneCountLabel);
        sceneHeaderLayout->addStretch();
        layout->addLayout(sceneHeaderLayout);

        sceneList = new QListWidget(this);
        sceneList->setAlternatingRowColors(true);
        layout->addWidget(sceneList);

        connect(sceneList, &QListWidget::itemDoubleClicked, this,
                &HueControlDialog::onSceneActivated);

        layout->addSpacing(10);

        // Sync control with FPS indicator in same row (no layout shift)
        auto syncHeaderLayout = new QHBoxLayout();
        syncButton = new QPushButton("Start Screen Sync", this);
        syncButton->setEnabled(true);
        syncButton->setIcon(QIcon::fromTheme("media-playback-start"));
        syncHeaderLayout->addWidget(syncButton);
        
        // FPS indicator (always present, empty when not syncing to prevent layout shift)
        fpsLabel = new QLabel("", this);
        fpsLabel->setStyleSheet("QLabel { color: green; font-size: 9pt; padding-left: 10px; }");
        fpsLabel->setMinimumWidth(150);  // Reserve space
        syncHeaderLayout->addWidget(fpsLabel);
        syncHeaderLayout->addStretch();

        layout->addLayout(syncHeaderLayout);

        connect(syncButton, &QPushButton::clicked, this, &HueControlDialog::onSyncToggled);

        layout->addSpacing(10);

        // Action buttons row
        auto buttonLayout = new QHBoxLayout();
        
        auto settingsBtn = new QPushButton(QIcon::fromTheme("configure"), "Select Room/Zone", this);
        connect(settingsBtn, &QPushButton::clicked, this, &HueControlDialog::onSettingsClicked);
        buttonLayout->addWidget(settingsBtn);

        auto refreshBtn = new QPushButton(QIcon::fromTheme("view-refresh"), "Refresh", this);
        connect(refreshBtn, &QPushButton::clicked, this, &HueControlDialog::refresh);
        buttonLayout->addWidget(refreshBtn);

        auto retryBtn = new QPushButton(QIcon::fromTheme("view-refresh"), "Retry Connection", this);
        retryBtn->hide(); // Show only when disconnected
        retryButton = retryBtn;
        connect(retryBtn, &QPushButton::clicked, this, &HueControlDialog::retryConnection);
        buttonLayout->addWidget(retryBtn);

        layout->addLayout(buttonLayout);

        // Initial connection state
        connectionState = CONNECTING;
        connectionRetryCount = 0;
        lastErrorShown = false;

        // Initial connection with retry
        QTimer::singleShot(100, this, &HueControlDialog::initialConnect);

        // Check connection status periodically
        connectionTimer = new QTimer(this);
        connect(connectionTimer, &QTimer::timeout, this, &HueControlDialog::checkConnectionStatus);
        connectionTimer->start(10000); // Check every 10 seconds

        // Check gaming mode status periodically
        gamingTimer = new QTimer(this);
        connect(gamingTimer, &QTimer::timeout, this, &HueControlDialog::updateGamingStatus);
        gamingTimer->start(2000); // Check every 2 seconds
    }

  public slots:
    void initialConnect() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            // Backend not running - retry with exponential backoff
            connectionRetryCount++;
            int delay = qMin(1000 * connectionRetryCount, 10000); // Max 10 seconds

            statusLabel->setText(QString("Connecting... (attempt %1)").arg(connectionRetryCount));
            updateConnectionState(CONNECTING);

            if (connectionRetryCount <= 5) {
                QTimer::singleShot(delay, this, &HueControlDialog::initialConnect);
            } else {
                // Give up after 5 attempts
                statusLabel->setText("Backend not available");
                updateConnectionState(DISCONNECTED);
                
                if (!lastErrorShown) {
                    showErrorNotification("Backend Not Running",
                                        "KDE Hue Control backend could not be reached.\n\n"
                                        "Start with: systemctl --user start hue-backend\n\n"
                                        "Or check if it's installed correctly.",
                                        KNotification::Persistent);
                    lastErrorShown = true;
                }
            }
            return;
        }

        // Backend is running - load data
        connectionState = CONNECTED;
        connectionRetryCount = 0;
        refresh();
    }

    void refresh() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            updateConnectionState(DISCONNECTED);
            statusLabel->setText("Backend not available");
            if (!lastErrorShown) {
                showErrorNotification("Service Unavailable",
                                    "Backend service is not running.\n\n"
                                    "Start with: systemctl --user start hue-backend",
                                    KNotification::Persistent);
                lastErrorShown = true;
            }
            return;
        }

        lastErrorShown = false;

        // Check connection status first
        checkConnectionStatus();

        // Get status
        QDBusReply<QString> statusReply = iface.call("GetStatus");
        if (statusReply.isValid()) {
            QString status = statusReply.value();
            if (connectionState == CONNECTED) {
                statusLabel->setText(status);
                updateConnectionState(CONNECTED);
            }
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

        // Get scenes with current scene indicator
        QDBusReply<QStringList> scenesReply = iface.call("GetScenes");
        if (scenesReply.isValid()) {
            QStringList scenes = scenesReply.value();
            sceneList->clear();
            
            QStringList filteredScenes;
            
            // Filter scenes by selected room if applicable
            if (!selectedRoom.isEmpty() && selectedRoom != "All Rooms") {
                for (const QString& scene : scenes) {
                    // Scene format: "Room Name - Scene Name"
                    if (scene.contains(" - ")) {
                        QString roomName = scene.left(scene.indexOf(" - "));
                        if (roomName == selectedRoom) {
                            filteredScenes << scene;
                        }
                    }
                }
            } else {
                filteredScenes = scenes;
            }
            
            // Add filtered scenes to list
            for (const QString& scene : filteredScenes) {
                QListWidgetItem* item = new QListWidgetItem(scene);
                sceneList->addItem(item);
            }
            
            sceneCountLabel->setText(QString("(%1 available)").arg(filteredScenes.count()));
        }

        // Get sync status
        QDBusReply<bool> syncReply = iface.call("IsSyncing");
        if (syncReply.isValid()) {
            bool syncing = syncReply.value();
            updateSyncButton(syncing);
            updateGamingStatus();
        }
    }

  private slots:
    void updateConnectionState(ConnectionState state) {
        connectionState = state;
        
        // Update visibility of retry button
        switch (state) {
            case CONNECTING:
            case CONNECTED:
                retryButton->hide();
                break;
            case DISCONNECTED:
            case ERROR:
                retryButton->show();
                break;
        }
    }

    void updateSyncButton(bool syncing) {
        // Always fetch real backend status to clear transient messages
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());
        if (iface.isValid()) {
            QDBusReply<QString> statusReply = iface.call("GetStatus");
            if (statusReply.isValid())
                statusLabel->setText(statusReply.value());
        }

        if (syncing) {
            syncButton->setText("Stop Screen Sync");
            syncButton->setIcon(QIcon::fromTheme("media-playback-stop"));
            fpsLabel->setText("30 FPS");
            fpsLabel->setStyleSheet("QLabel { color: green; font-size: 9pt; padding-left: 10px; }");
        } else {
            syncButton->setText("Start Screen Sync");
            syncButton->setIcon(QIcon::fromTheme("media-playback-start"));
            fpsLabel->setText("");
        }
    }

    void updatePresetButtons(int value) {
        // Deselect all preset buttons first
        preset25Button->setChecked(false);
        preset50Button->setChecked(false);
        preset75Button->setChecked(false);
        preset100Button->setChecked(false);

        // Select the button that matches the current value
        if (value == 25) {
            preset25Button->setChecked(true);
        } else if (value == 50) {
            preset50Button->setChecked(true);
        } else if (value == 75) {
            preset75Button->setChecked(true);
        } else if (value == 100) {
            preset100Button->setChecked(true);
        }
    }

    void onPowerToggled(bool checked) {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());
        
        statusLabel->setText(checked ? "⏳ Turning on..." : "⏳ Turning off...");
        powerCheckbox->setEnabled(false);
        
        QDBusReply<bool> reply = iface.call("SetPower", checked);
        
        powerCheckbox->setEnabled(true);
        
        if (!reply.isValid() || !reply.value()) {
            // Revert checkbox on failure
            powerCheckbox->blockSignals(true);
            powerCheckbox->setChecked(!checked);
            powerCheckbox->blockSignals(false);
            
            showErrorNotification("Power Control Failed",
                                "Failed to turn " + QString(checked ? "on" : "off") + " lights.\n\n"
                                "Bridge may be unreachable or lights are offline.",
                                KNotification::CloseOnTimeout);
        } else {
            // Success - show brief confirmation
            statusLabel->setText("Power " + QString(checked ? "On" : "Off"));
        }

        // Refresh state after a short delay to get updated brightness
        QTimer::singleShot(500, this, &HueControlDialog::refresh);
    }

    void onBrightnessChanged(int value) {
        brightnessValueLabel->setText(QString::number(value) + "%");
        brightnessValueLabel->setStyleSheet(QString("QLabel { font-weight: bold; color: %1; }")
                                          .arg(value > 75 ? "green" : value > 25 ? "orange" : "gray"));

        // Update preset button states
        updatePresetButtons(value);

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
        
        QDBusReply<bool> reply = iface.call("SetBrightness", pendingBrightness);
        
        if (!reply.isValid() || !reply.value()) {
            // Brightness change failed - show error but don't revert slider
            // (might be just a temporary network glitch)
            qDebug() << "Brightness change failed:" << reply.error().message();
        } else {
            statusLabel->setText(QString("Brightness set to %1%").arg(pendingBrightness));
        }
    }

    void onSceneActivated(QListWidgetItem* item) {
        QString sceneName = item->text();
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            statusLabel->setText("Backend not available");
            updateConnectionState(DISCONNECTED);
            showErrorNotification("Service Unavailable",
                                "Backend service is not running.\n\n"
                                "Start with: systemctl --user start hue-backend",
                                KNotification::Persistent);
            return;
        }

        // Show loading state with visual feedback
        statusLabel->setText("⏳ Activating scene: " + sceneName);
        sceneList->setEnabled(false);
        item->setIcon(QIcon::fromTheme("emblem-synchronizing"));

        // Make async call to avoid blocking UI
        QDBusPendingCall call = iface.asyncCall("ActivateScene", sceneName);
        QDBusPendingCallWatcher* watcher = new QDBusPendingCallWatcher(call, this);

        connect(watcher, &QDBusPendingCallWatcher::finished, this,
                [this, sceneName, item](QDBusPendingCallWatcher* w) {
                    sceneList->setEnabled(true);
                    QDBusPendingReply<QString> reply = *w;

                    if (reply.isError()) {
                        QString error = reply.error().message();
                        statusLabel->setText("Failed to activate scene");
                        item->setIcon(QIcon::fromTheme("dialog-error"));

                        // Provide user-friendly error messages
                        if (error.contains("unreachable") || error.contains("timeout") || 
                            error.contains("connection refused")) {
                            showErrorNotification(
                                "Bridge Unreachable",
                                "Cannot connect to Hue Bridge.\n\n"
                                "Check:\n"
                                "• Bridge is powered on\n"
                                "• Network connection is working\n"
                                "• Bridge IP in config is correct\n\n"
                                "Bridge IP: Check ~/.openhue/config.yaml",
                                KNotification::Persistent);
                        } else if (error.contains("not found") || error.contains("unknown")) {
                            showErrorNotification("Scene Not Found",
                                                "Scene '" + sceneName + "' could not be found.\n\n"
                                                "It may have been deleted. Refresh the scene list.",
                                                KNotification::CloseOnTimeout);
                        } else {
                            showErrorNotification("Scene Activation Failed",
                                                "Failed to activate: " + sceneName + "\n\n"
                                                "Error: " + error + "\n\n"
                                                "Try again or check bridge status.",
                                                KNotification::CloseOnTimeout);
                        }

                        // Reset icon after 2 seconds
                        QTimer::singleShot(2000, [item]() {
                            item->setIcon(QIcon::fromTheme("favorites"));
                        });
                    } else {
                        QString result = reply.value();
                        statusLabel->setText("Scene activated: " + sceneName);
                        item->setIcon(QIcon::fromTheme("emblem-checked"));

                        // Show success notification with icon
                        KNotification* notif = new KNotification("sceneActivated");
                        notif->setTitle("Scene Activated");
                        notif->setText(sceneName);
                        notif->setIconName("preferences-desktop-display-color");
                        notif->setUrgency(KNotification::LowUrgency);
                        notif->sendEvent();

                        // Reset icon after 2 seconds
                        QTimer::singleShot(2000, [item]() {
                            item->setIcon(QIcon::fromTheme("favorites"));
                        });
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
            syncButton->setIcon(QIcon::fromTheme("process-stop"));

            QDBusReply<bool> reply = iface.call("StopSync");
            if (reply.isValid() && reply.value()) {
                updateSyncButton(false);

                // Show notification with action
                KNotification* notif = new KNotification("syncStopped");
                notif->setTitle("Screen Sync Stopped");
                notif->setText("Lights are no longer syncing with screen");
                notif->setIconName("media-playback-stop");
                notif->setUrgency(KNotification::LowUrgency);
                notif->sendEvent();
            } else {
                syncButton->setText("Stop Screen Sync");
                showErrorNotification("Failed to Stop Sync",
                                    reply.isValid() ? reply.error().message() : "Unknown error",
                                    KNotification::CloseOnTimeout);
            }
            syncButton->setEnabled(true);
        } else {
            // Starting sync
            syncButton->setEnabled(false);
            syncButton->setText("⏳ Starting...");
            syncButton->setIcon(QIcon::fromTheme("chronometer"));
            fpsLabel->setText("");  // Clear text
            statusLabel->setText("Waiting for screen share approval");

            // Show info about permission dialog
            KNotification* permNotif = new KNotification("syncPermission");
            permNotif->setTitle("Screen Sharing Permission Required");
            permNotif->setText("Please select your monitor and click 'Share' in the dialog that appears.");
            permNotif->setIconName("dialog-information");
            permNotif->setUrgency(KNotification::LowUrgency);
            permNotif->sendEvent();

            QDBusPendingCall call = iface.asyncCall("StartSync");
            QDBusPendingCallWatcher* watcher = new QDBusPendingCallWatcher(call, this);

            connect(watcher, &QDBusPendingCallWatcher::finished, this,
                    [this](QDBusPendingCallWatcher* w) {
                        syncButton->setEnabled(true);
                        QDBusPendingReply<bool> reply = *w;

                        if (reply.isError() || !reply.value()) {
                            QString error = reply.isValid() ? reply.error().message() : "Unknown error";
                            updateSyncButton(false);

                            // Parse portal errors for user-friendly messages
                            if (error.contains("PortalError:permission_denied")) {
                                QString hint = error.section(':', 2);

                                KNotification* notif = new KNotification("syncFailed");
                                notif->setTitle("Screen Sharing Permission Denied");
                                notif->setText(
                                    "You must approve the screen sharing dialog.\n\n" +
                                    hint + "\n\n"
                                    "Click 'Start Screen Sync' to try again.");
                                notif->setIconName("dialog-warning");
                                notif->setUrgency(KNotification::NormalUrgency);
                                notif->sendEvent();
                            } else if (error.contains("PortalError:")) {
                                QString errorType = error.section(':', 1, 1);
                                QString hint = error.section(':', 2);

                                showErrorNotification("Screen Sync Failed",
                                                    "Portal Error: " + errorType + "\n\n" + hint +
                                                    "\n\nCheck that xdg-desktop-portal is running.",
                                                    KNotification::Persistent);
                            } else if (error.contains("sync engine not available") || 
                                     error.contains("Entertainment") || 
                                     error.contains("clientkey")) {
                                showErrorNotification("Screen Sync Not Configured",
                                                    "Entertainment API is not configured.\n\n"
                                                    "Setup required:\n"
                                                    "1. Create Entertainment Area in Hue app\n"
                                                    "2. Configure clientkey in ~/.openhue/config.yaml\n"
                                                    "3. Set EntertainmentConfigurationID\n\n"
                                                    "See documentation for details.",
                                                    KNotification::Persistent);
                            } else if (error.contains("PipeWire") || error.contains("capture")) {
                                showErrorNotification("Screen Capture Failed",
                                                    "Failed to start screen capture.\n\n"
                                                    "Check:\n"
                                                    "• PipeWire is running\n"
                                                    "• xdg-desktop-portal-kde is installed\n"
                                                    "• You approved the permission dialog\n\n"
                                                    "Error: " + error,
                                                    KNotification::Persistent);
                            } else {
                                showErrorNotification("Failed to Start Screen Sync",
                                                    error + "\n\n"
                                                    "Check backend logs:\n"
                                                    "journalctl --user -u hue-backend -n 50",
                                                    KNotification::Persistent);
                            }
                        } else {
                            updateSyncButton(true);

                            // Show success notification
                            KNotification* notif = new KNotification("syncStarted");
                            notif->setTitle("Screen Sync Started");
                            notif->setText("Lights are now syncing with your screen at 30 FPS");
                            notif->setIconName("media-record");
                            notif->setUrgency(KNotification::LowUrgency);
                            notif->sendEvent();
                        }

                        w->deleteLater();
                    });
        }
    }

    void onSettingsClicked() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            QMessageBox::warning(this, "Service Unavailable", 
                               "Backend service is not running.");
            return;
        }

        // Get all scenes to extract room names
        QDBusReply<QStringList> scenesReply = iface.call("GetScenes");
        if (!scenesReply.isValid()) {
            QMessageBox::warning(this, "Error", "Failed to get scenes from backend.");
            return;
        }

        QStringList scenes = scenesReply.value();
        QSet<QString> roomSet;
        
        // Extract unique room names from scenes (format: "Room Name - Scene Name")
        for (const QString& scene : scenes) {
            if (scene.contains(" - ")) {
                QString roomName = scene.left(scene.indexOf(" - "));
                roomSet.insert(roomName);
            }
        }

        if (roomSet.isEmpty()) {
            QMessageBox::information(this, "No Rooms", 
                                   "No rooms found. Scenes must be in format 'Room Name - Scene Name'.");
            return;
        }

        // Build sorted list of rooms with "All Rooms" at the top
        QStringList roomList;
        roomList << "All Rooms";
        QStringList sortedRooms = roomSet.values();
        sortedRooms.sort();
        roomList << sortedRooms;

        // Show selection dialog
        bool ok;
        int currentIndex = 0;
        if (!selectedRoom.isEmpty()) {
            currentIndex = roomList.indexOf(selectedRoom);
            if (currentIndex < 0) currentIndex = 0;
        }
        
        QString selected = QInputDialog::getItem(
            this, "Select Room/Zone", "Choose a room to filter scenes:", 
            roomList, currentIndex, false, &ok);

        if (ok && !selected.isEmpty()) {
            selectedRoom = selected;
            
            // Refresh scene list with new filter
            refresh();
            
            // Show confirmation
            QString message = (selected == "All Rooms") 
                ? "Showing scenes from all rooms" 
                : QString("Filtering scenes for: %1").arg(selected);
            statusLabel->setText(message);
        }
    }

    void checkConnectionStatus() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        if (!iface.isValid()) {
            updateConnectionState(DISCONNECTED);
            return; // Service not running
        }

        QDBusReply<QVariantMap> reply = iface.call("GetConnectionStatus");
        if (!reply.isValid()) {
            // Method not available (old backend version) - assume connected
            return;
        }

        QVariantMap status = reply.value();
        bool connected = status["connected"].toBool();
        QString lastError = status["lastError"].toString();
        QString bridgeIP = status["bridgeIP"].toString();

        if (!connected && !lastError.isEmpty()) {
            // Bridge is unreachable
            updateConnectionState(ERROR);
            statusLabel->setText("Bridge unreachable: " + bridgeIP);
            
            // Show detailed connection info
            connectionDetailsLabel->setText(
                QString("Last error: %1\nBridge IP: %2\nCheck network and bridge power")
                    .arg(lastError).arg(bridgeIP));
            connectionDetailsLabel->show();

            // Show notification once per disconnection
            static QString lastErrorTime;
            QString currentTime = QDateTime::currentDateTime().toString(Qt::ISODate);
            
            if (lastErrorTime != currentTime.left(16)) { // Check per minute
                lastErrorTime = currentTime.left(16);
                
                KNotification* notif = new KNotification("connectionFailed");
                notif->setTitle("Hue Bridge Unreachable");
                notif->setText(QString("Cannot connect to bridge at %1\n\n%2\n\n"
                                       "Open the control panel and click 'Retry' to reconnect.")
                                   .arg(bridgeIP)
                                   .arg(lastError));
                notif->setIconName("network-disconnect");
                notif->setUrgency(KNotification::NormalUrgency);
                notif->sendEvent();
            }
        } else if (connected) {
            // Connection restored
            static bool wasDisconnected = false;
            if (connectionState == ERROR || connectionState == DISCONNECTED) {
                wasDisconnected = true;
            }
            
            if (wasDisconnected) {
                wasDisconnected = false;
                updateConnectionState(CONNECTED);
                statusLabel->setText("Connection restored");
                connectionDetailsLabel->hide();

                KNotification* notif = new KNotification("connectionRestored");
                notif->setTitle("Bridge Connection Restored");
                notif->setText("Successfully reconnected to Hue Bridge at " + bridgeIP);
                notif->setIconName("network-connect");
                notif->setUrgency(KNotification::LowUrgency);
                notif->sendEvent();

                refresh(); // Reload scenes and state
            } else {
                // Normal connected state
                updateConnectionState(CONNECTED);
                connectionDetailsLabel->hide();
            }
        }
    }

    void retryConnection() {
        QDBusInterface iface("org.kde.plasma.hue", "/org/kde/plasma/hue", "org.kde.plasma.hue",
                             QDBusConnection::sessionBus());

        statusLabel->setText("Retrying connection...");
        updateConnectionState(CONNECTING);
        connectionDetailsLabel->hide();

        QDBusReply<bool> reply = iface.call("RetryConnection");
        if (reply.isValid() && reply.value()) {
            statusLabel->setText("Connection restored");
            updateConnectionState(CONNECTED);

            KNotification* notif = new KNotification("connectionRestored");
            notif->setTitle("Connection Restored");
            notif->setText("Successfully reconnected to Hue Bridge");
            notif->setIconName("network-connect");
            notif->setUrgency(KNotification::LowUrgency);
            notif->sendEvent();

            refresh();
        } else {
            statusLabel->setText("Still unreachable");
            updateConnectionState(ERROR);
            
            showErrorNotification("Retry Failed",
                                "Bridge is still unreachable.\n\n"
                                "Check:\n"
                                "• Bridge is powered on\n"
                                "• Network connection is working\n"
                                "• Bridge IP in config is correct (~/.openhue/config.yaml)",
                                KNotification::Persistent);
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

        // Update FPS label based on sync state
        if (syncing && gamingActive) {
            fpsLabel->setText("30 FPS (Gaming)");
            fpsLabel->setStyleSheet("QLabel { color: #00ff00; font-size: 9pt; padding-left: 10px; font-weight: bold; }");
        } else if (syncing) {
            fpsLabel->setText("30 FPS");
            fpsLabel->setStyleSheet("QLabel { color: green; font-size: 9pt; padding-left: 10px; }");
        } else {
            fpsLabel->setText("");
        }
    }

    void showErrorNotification(const QString& title, const QString& message, 
                              KNotification::NotificationFlags flags = KNotification::CloseOnTimeout) {
        KNotification* notif = new KNotification("error");
        notif->setTitle(title);
        notif->setText(message);
        notif->setIconName("dialog-error");
        notif->setUrgency(KNotification::NormalUrgency);
        notif->setFlags(flags);
        notif->sendEvent();
    }

  private:
    // UI elements
    QLabel* statusLabel;
    QLabel* connectionDetailsLabel;
    QCheckBox* powerCheckbox;
    QSlider* brightnessSlider;
    QLabel* brightnessValueLabel;
    QPushButton* preset25Button;
    QPushButton* preset50Button;
    QPushButton* preset75Button;
    QPushButton* preset100Button;
    QListWidget* sceneList;
    QLabel* sceneCountLabel;
    QPushButton* syncButton;
    QLabel* fpsLabel;
    QPushButton* retryButton;
    
    // Timers
    QTimer* brightnessTimer = nullptr;
    QTimer* connectionTimer;
    QTimer* gamingTimer;
    
    // State
    int pendingBrightness = 100;
    ConnectionState connectionState;
    int connectionRetryCount;
    bool lastErrorShown;
    QString selectedRoom; // For room filtering
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

        QDBusMessage reply = iface.call("GetTrayIcons");
        if (reply.type() == QDBusMessage::ErrorMessage) {
            // Method failed, keep defaults
            qDebug() << "Failed to get tray icons:" << reply.errorMessage();
            return;
        }

        // GetTrayIcons returns (string gaming, string syncing, string idle)
        // Parse as three separate QVariant strings in reply.arguments() list
        // (not as QDBusVariant or struct - that was the initial bug)
        QList<QVariant> args = reply.arguments();
        if (args.size() >= 3) {
            gamingIconName = args[0].toString();
            syncingIconName = args[1].toString();
            idleIconName = args[2].toString();
            qDebug() << "Loaded icon names - Gaming:" << gamingIconName 
                     << "Syncing:" << syncingIconName 
                     << "Idle:" << idleIconName;
        }
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
        // Icons are cached member variables loaded from backend config (not hardcoded)
        // This allows users to customize icons via settings dialog
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
            tooltipText = "Gaming Mode Active - Syncing to screen";
        } else if (syncing) {
            tooltipText = "Syncing lights to screen";
        } else if (gamingEnabled && gamingActive) {
            tooltipText = "Game detected - preparing to sync...";
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

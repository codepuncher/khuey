#include "huebackend.h"
#include "settingsdialog.h"
#include <KNotification>
#include <KStatusNotifierItem>
#include <QAction>
#include <QApplication>
#include <QCheckBox>
#include <QDBusAbstractInterface>
#include <QDBusArgument>
#include <QDBusMessage>
#include <QDBusPendingCall>
#include <QDBusPendingReply>
#include <QDBusReply>
#include <QDBusServiceWatcher>
#include <QDateTime>
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
#include <QPointer>
#include <QProcess>
#include <QPushButton>
#include <QSet>
#include <QSlider>
#include <QTimer>
#include <QVBoxLayout>
#include <QWidget>
#include <algorithm>
#include <functional>
#include <memory>

// Enum for connection state
enum ConnectionState { CONNECTING, CONNECTED, DISCONNECTED, ERROR };

/**
 * The backend gives the portal's screen-share dialog two minutes to be
 * answered, then spends up to 40 more on the bridge activation and the DTLS
 * handshake. Qt's 25-second default reply timeout gives up while the start is
 * still running and reports a failure for a sync that goes on to succeed.
 */
constexpr int startSyncTimeoutMs = 3 * 60 * 1000;

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
        connectionDetailsLabel->setStyleSheet(
            "QLabel { color: gray; font-size: 10pt; padding: 5px; }");
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
        connect(preset25Button, &QPushButton::clicked,
                [this]() { brightnessSlider->setValue(25); });
        connect(preset50Button, &QPushButton::clicked,
                [this]() { brightnessSlider->setValue(50); });
        connect(preset75Button, &QPushButton::clicked,
                [this]() { brightnessSlider->setValue(75); });
        connect(preset100Button, &QPushButton::clicked,
                [this]() { brightnessSlider->setValue(100); });

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

        auto sceneActionLayout = new QHBoxLayout();
        activateSceneBtn =
            new QPushButton(QIcon::fromTheme("media-playback-start"), "Activate Scene", this);
        activateSceneBtn->setEnabled(false);
        sceneActionLayout->addWidget(activateSceneBtn);

        syncButton = new QPushButton("Start Screen Sync", this);
        syncButton->setEnabled(true);
        syncButton->setIcon(QIcon::fromTheme("media-record"));
        sceneActionLayout->addWidget(syncButton);

        layout->addLayout(sceneActionLayout);

        connect(sceneList, &QListWidget::itemSelectionChanged, this, [this]() {
            // A refresh reply re-selects the previous item even while the list is disabled
            activateSceneBtn->setEnabled(sceneList->isEnabled() && sceneList->currentItem());
        });
        connect(sceneList, &QListWidget::itemDoubleClicked, this,
                &HueControlDialog::onSceneActivated);
        connect(activateSceneBtn, &QPushButton::clicked, this, [this]() {
            if (sceneList->currentItem()) {
                onSceneActivated(sceneList->currentItem());
            }
        });
        connect(syncButton, &QPushButton::clicked, this, &HueControlDialog::onSyncToggled);

        layout->addSpacing(10);

        // Action buttons row
        auto buttonLayout = new QHBoxLayout();

        settingsButton = new QPushButton(QIcon::fromTheme("configure"), "Select Room/Zone", this);
        connect(settingsButton, &QPushButton::clicked, this, &HueControlDialog::onSettingsClicked);
        buttonLayout->addWidget(settingsButton);

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

        // Initial connection with retry
        QTimer::singleShot(100, this, &HueControlDialog::initialConnect);

        /**
         * Periodically poll connection and gaming/sync status. Deliberately
         * lightweight (no scene list rebuild, no brightness/power reload) so it
         * can't race a manual refresh() or clobber an in-flight brightness edit
         * or scene activation. checkConnectionStatus() calls refresh() itself
         * on the rare reconnect transition, so scenes/state still get reloaded
         * when the bridge comes back.
         */
        connectionTimer = new QTimer(this);
        connect(connectionTimer, &QTimer::timeout, this, &HueControlDialog::pollStatus);
        connectionTimer->start(10000); // Check every 10 seconds

        /**
         * Polling stops while the panel is hidden, so the bus is the only
         * thing left that can tell a standing "backend missing" popup to
         * close.
         */
        auto* backendWatcher = new QDBusServiceWatcher(
            iface.service(), iface.connection(), QDBusServiceWatcher::WatchForRegistration, this);
        connect(backendWatcher, &QDBusServiceWatcher::serviceRegistered, this,
                [this]() { clearStateError(backendMissing); });
    }

  protected:
    void showEvent(QShowEvent* event) override {
        connectionTimer->start(10000);
        refresh();
        QDialog::showEvent(event);
    }

    void hideEvent(QHideEvent* event) override {
        connectionTimer->stop();
        QDialog::hideEvent(event);
    }

  public slots:
    void initialConnect() {
        if (!iface.isValid()) {
            // Backend not running - retry with linear backoff
            connectionRetryCount++;
            int delay = qMin(1000 * connectionRetryCount, 10000);

            statusLabel->setText(QString("Connecting... (attempt %1)").arg(connectionRetryCount));
            updateConnectionState(CONNECTING);

            if (connectionRetryCount <= 5) {
                QTimer::singleShot(delay, this, &HueControlDialog::initialConnect);
            } else {
                // Give up after 5 attempts
                statusLabel->setText("Backend not available");
                updateConnectionState(DISCONNECTED);

                showStateError(backendMissing, "Backend Not Running",
                               "KDE Hue Control backend could not be reached.\n\n"
                               "Start with: systemctl --user start hue-backend\n\n"
                               "Or check if it's installed correctly.");
            }
            return;
        }

        // Backend is running - load data
        connectionState = CONNECTED;
        connectionRetryCount = 0;
        refresh();
    }

    /**
     * Lightweight periodic poll: connection health plus gaming/sync status
     * only. Does not touch scenes, power, brightness, or FPS - those are
     * reloaded by refresh(), which this deliberately avoids calling on every
     * tick to prevent racing an in-flight brightness edit or scene activation.
     */
    void pollStatus() {
        if (!iface.isValid()) {
            updateConnectionState(DISCONNECTED);
            statusLabel->setText("Backend not available");
            statusAfterRefresh.clear();
            return;
        }

        if (pollPending) {
            return;
        }
        pollPending = true;

        const int stateWritesAtStart = stateWrites;
        QDBusPendingCall connectionCall = iface.asyncCall("GetConnectionStatus");
        QDBusPendingCall syncCall = iface.asyncCall("IsSyncing");
        QDBusPendingCall gamingCall = iface.asyncCall("IsGamingModeActive");

        auto apply = [this, connectionCall, syncCall, gamingCall, stateWritesAtStart]() {
            pollPending = false;

            if (checkConnectionStatus(connectionCall)) {
                /**
                 * Just reconnected - refresh() reloads scenes/state and covers
                 * gaming/sync status too, so there's nothing left to do here.
                 */
                refresh();
                return;
            }

            if (stateChangedSince(stateWritesAtStart)) {
                return;
            }

            QDBusReply<bool> syncReply = syncCall;
            QDBusReply<bool> gamingReply = gamingCall;
            updateGamingStatus(syncReply.isValid() && syncReply.value(),
                               gamingReply.isValid() && gamingReply.value());
        };
        whenFinished(this, {connectionCall, syncCall, gamingCall}, apply);
    }

    void refresh() {
        if (!iface.isValid()) {
            updateConnectionState(DISCONNECTED);
            statusLabel->setText("Backend not available");
            statusAfterRefresh.clear();
            showStateError(backendMissing, "Backend Not Running",
                           "Backend service is not running.\n\n"
                           "Start with: systemctl --user start hue-backend");
            return;
        }

        clearStateError(backendMissing);

        if (refreshPending) {
            refreshQueued = true;
            return;
        }
        refreshPending = true;
        refreshStale = false;

        const int stateWritesAtStart = stateWrites;
        QDBusPendingCall connectionCall = iface.asyncCall("GetConnectionStatus");
        QDBusPendingCall statusCall = iface.asyncCall("GetStatus");
        QDBusPendingCall syncCall = iface.asyncCall("IsSyncing");
        QDBusPendingCall syncSettingsCall = iface.asyncCall("GetSyncSettings");
        QDBusPendingCall stateCall = iface.asyncCall("GetState");
        QDBusPendingCall scenesCall = iface.asyncCallWithTimeout("GetScenes", getScenesTimeoutMs);
        QDBusPendingCall gamingCall = iface.asyncCall("IsGamingModeActive");

        auto apply = [this, connectionCall, statusCall, syncCall, syncSettingsCall, stateCall,
                      scenesCall, gamingCall, stateWritesAtStart]() {
            refreshPending = false;
            const bool stale = stateChangedSince(stateWritesAtStart);

            // Sets the connectionState the status block below reads
            checkConnectionStatus(connectionCall);

            // Get status - but don't overwrite meaningful state (active scene or sync)
            QDBusReply<QString> statusReply = statusCall;

            // Get sync status once for use in multiple places
            QDBusReply<bool> syncReply = syncCall;
            bool syncing = syncReply.isValid() && syncReply.value();

            // Refresh the configured FPS so status text reflects the real sync rate
            QDBusReply<QVariantMap> syncSettingsReply = syncSettingsCall;
            int fps =
                syncSettingsReply.isValid() ? syncSettingsReply.value().value("fps").toInt() : 0;
            if (fps > 0) {
                currentFps = fps;
            }

            if (statusReply.isValid() && !stale && connectionState == CONNECTED) {
                updateConnectionState(CONNECTED);
                // Update status label based on current state
                if (syncing) {
                    activeScene.clear();
                    statusLabel->setText(QString("Screen sync active  •  %1 FPS").arg(currentFps));
                } else if (!activeScene.isEmpty()) {
                    statusLabel->setText("Scene: " + activeScene);
                } else {
                    statusLabel->setText(statusReply.value());
                }
            }

            // Get current state (power and brightness)
            QDBusMessage stateMsg = stateCall.reply();
            if (stateMsg.type() == QDBusMessage::ReplyMessage && stateMsg.arguments().size() >= 3) {
                bool power = stateMsg.arguments().at(0).toBool();
                int brightness = stateMsg.arguments().at(1).toInt();
                bool success = stateMsg.arguments().at(2).toBool();

                if (success && !stale) {
                    // Block signals while updating to avoid triggering DBus calls
                    powerCheckbox->blockSignals(true);
                    brightnessSlider->blockSignals(true);

                    powerCheckbox->setChecked(power);
                    brightnessSlider->setValue(brightness);
                    brightnessValueLabel->setText(QString::number(brightness) + "%");
                    updatePresetButtons(brightness);

                    // Update pendingBrightness to match actual state
                    pendingBrightness = brightness;

                    powerCheckbox->blockSignals(false);
                    brightnessSlider->blockSignals(false);
                }
            }

            // Get scenes with current scene indicator
            QDBusReply<QStringList> scenesReply = scenesCall;
            if (scenesReply.isValid()) {
                QStringList scenes = scenesReply.value();

                // Preserve the current selection across the reload
                QString previousSelection;
                if (QListWidgetItem* current = sceneList->currentItem()) {
                    previousSelection = current->text();
                }

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

                // Add filtered scenes to list, restoring the previous selection if still present
                for (const QString& scene : filteredScenes) {
                    QListWidgetItem* item = new QListWidgetItem(scene);
                    sceneList->addItem(item);
                    if (scene == previousSelection) {
                        sceneList->setCurrentItem(item);
                    }
                }

                sceneCountLabel->setText(QString("(%1 available)").arg(filteredScenes.count()));
            }

            QDBusReply<bool> gamingReply = gamingCall;
            if (!stale) {
                updateGamingStatus(syncing, gamingReply.isValid() && gamingReply.value());
            }

            // A stale or queued refresh is followed by one whose status text would replace it
            if (!statusAfterRefresh.isEmpty() && !stale && !refreshQueued) {
                statusLabel->setText(statusAfterRefresh);
                statusAfterRefresh.clear();
            }

            refreshStale = stale;
            if (refreshQueued) {
                refreshQueued = false;
                refresh();
            }
            refreshIfStale();
        };
        whenFinished(this,
                     {connectionCall, statusCall, syncCall, syncSettingsCall, stateCall, scenesCall,
                      gamingCall},
                     apply);
    }

  private slots:
    void updateConnectionState(ConnectionState state) {
        connectionState = state;

        switch (state) {
            case CONNECTED:
                clearStateError(bridgeUnreachable);
                retryButton->hide();
                break;
            case CONNECTING:
                retryButton->hide();
                break;
            case DISCONNECTED:
            case ERROR:
                retryButton->show();
                break;
        }

        updateControlsEnabled();
    }

    /**
     * Composes every reason a control is unusable: no backend to answer the
     * call, sync owning the lights, or that control's own call still in
     * flight. Re-enabling always goes through here, so a periodic
     * pollStatus()/refresh() tick can't hand back a control another reason
     * still holds. Disabling one directly is safe and stays where it is.
     *
     * Backend presence comes from the interface rather than connectionState,
     * which a retry moves to CONNECTING before anything has answered.
     *
     * Bridge trouble (ERROR) leaves the controls alone: the backend still
     * answers, the failure notifications already name the bridge, and the
     * retry that clears it lives in the same panel.
     */
    void updateControlsEnabled() {
        bool backendUp = iface.isValid();
        bool lightControls = backendUp && !syncActive;

        brightnessSlider->setEnabled(lightControls);
        brightnessValueLabel->setEnabled(lightControls);
        preset25Button->setEnabled(lightControls);
        preset50Button->setEnabled(lightControls);
        preset75Button->setEnabled(lightControls);
        preset100Button->setEnabled(lightControls);
        powerCheckbox->setEnabled(lightControls && !powerChangePending);

        bool sceneControlsEnabled = lightControls && !sceneActivationPending;
        sceneList->setEnabled(sceneControlsEnabled);
        activateSceneBtn->setEnabled(sceneControlsEnabled && sceneList->currentItem());

        syncButton->setEnabled(backendUp && !syncTogglePending);
        settingsButton->setEnabled(backendUp && !roomPickerPending);
    }

    void updateSyncButton(bool syncing) {
        syncActive = syncing;
        if (syncing) {
            syncButton->setText("Stop Screen Sync");
            syncButton->setIcon(QIcon::fromTheme("media-playback-stop"));
        } else {
            syncButton->setText("Start Screen Sync");
            syncButton->setIcon(QIcon::fromTheme("media-playback-start"));
        }

        updateControlsEnabled();
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
        statusLabel->setText(checked ? "Turning on..." : "Turning off...");
        powerCheckbox->setEnabled(false);
        powerChangePending = true;
        queueLightWrite(false, [this, checked]() { sendPower(checked); });
    }

    void sendPower(bool checked) {
        QDBusPendingCall call = iface.asyncCall("SetPower", checked);
        whenFinished(this, {call}, [this, call, checked]() {
            powerChangePending = false;
            updateControlsEnabled();

            QDBusReply<bool> reply = call;
            if (!reply.isValid() || !reply.value()) {
                // Revert checkbox on failure
                powerCheckbox->blockSignals(true);
                powerCheckbox->setChecked(!checked);
                powerCheckbox->blockSignals(false);

                showErrorNotification("Power Control Failed",
                                      "Failed to turn " + QString(checked ? "on" : "off") +
                                          " lights.\n\n"
                                          "Bridge may be unreachable or lights are offline.",
                                      KNotification::CloseOnTimeout);
            } else {
                // Success - clear active scene (power change invalidates it) and show confirmation
                activeScene.clear();
                showWriteResult("Power " + QString(checked ? "On" : "Off"));
            }

            // Refresh state after a short delay to get updated brightness
            QTimer::singleShot(500, this, &HueControlDialog::refresh);
            lightWriteFinished();
        });
    }

    void onBrightnessChanged(int value) {
        // Brightness no longer matches whatever scene was last activated
        activeScene.clear();

        brightnessValueLabel->setText(QString::number(value) + "%");
        brightnessValueLabel->setStyleSheet(QString("QLabel { font-weight: bold; color: %1; }")
                                                .arg(value > 75   ? "green"
                                                     : value > 25 ? "orange"
                                                                  : "gray"));

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
        // A queued brightness write reads pendingBrightness when it starts
        if (!lightWrites.isEmpty() && lightWrites.last().brightness) {
            return;
        }
        queueLightWrite(true, [this]() { sendBrightness(); });
    }

    void sendBrightness() {
        const int brightness = pendingBrightness;
        QDBusPendingCall call = iface.asyncCall("SetBrightness", brightness);
        whenFinished(this, {call}, [this, call, brightness]() {
            QDBusReply<bool> reply = call;
            if (!reply.isValid() || !reply.value()) {
                /**
                 * Brightness change failed - show error but don't revert slider
                 * (might be just a temporary network glitch)
                 */
                qDebug() << "Brightness change failed:" << reply.error().message();
            } else {
                // A scene queued ahead of this write has set activeScene since the drag cleared it
                activeScene.clear();
                showWriteResult(QString("Brightness set to %1%").arg(brightness));
            }

            /**
             * A brightness above 0 turns the group on at the bridge, so the
             * panel's power state is stale until this lands.
             */
            QTimer::singleShot(500, this, &HueControlDialog::refresh);
            lightWriteFinished();
        });
    }

    /**
     * The backend runs each call on its own goroutine, so writes sent
     * together can reach the bridge in any order. Light writes go out one at
     * a time, in the order the user made them.
     */
    void queueLightWrite(bool brightness, std::function<void()> start) {
        // A brightness change still in its debounce was made first
        if (!brightness && brightnessTimer && brightnessTimer->isActive()) {
            brightnessTimer->stop();
            applyBrightness();
        }
        lightWrites.append({brightness, start});
        if (lightWriteInFlight) {
            return;
        }
        startNextLightWrite();
    }

    void startNextLightWrite() {
        lightWriteInFlight = !lightWrites.isEmpty();
        if (!lightWriteInFlight) {
            refreshIfStale();
            return;
        }
        lightWrites.takeFirst().start();
    }

    /**
     * A queued power or scene write has already put its own progress text in
     * the label. Brightness writes show none.
     */
    void showWriteResult(const QString& text) {
        bool progressShown = std::any_of(lightWrites.cbegin(), lightWrites.cend(),
                                         [](const LightWrite& w) { return !w.brightness; });
        if (progressShown) {
            return;
        }
        statusLabel->setText(text);
    }

    void lightWriteFinished() {
        stateWrites++;
        startNextLightWrite();
    }

    bool stateChangePending() const {
        return lightWriteInFlight || syncTogglePending ||
               (brightnessTimer && brightnessTimer->isActive());
    }

    /**
     * A reply can predate a power, brightness, scene or sync change made
     * while it was in flight, and applying it would snap the controls back.
     */
    bool stateChangedSince(int writesAtStart) const {
        return stateWrites != writesAtStart || stateChangePending();
    }

    /**
     * Re-runs a refresh whose replies were dropped as stale, once nothing
     * that could make them stale again is pending.
     */
    void refreshIfStale() {
        if (!refreshStale || stateChangePending()) {
            return;
        }
        refresh();
    }

    void onSceneActivated(QListWidgetItem* item) {
        QString sceneName = item->text();

        if (!iface.isValid()) {
            statusLabel->setText("Backend not available");
            updateConnectionState(DISCONNECTED);
            showStateError(backendMissing, "Backend Not Running",
                           "Backend service is not running.\n\n"
                           "Start with: systemctl --user start hue-backend");
            return;
        }

        // Show loading state with visual feedback
        statusLabel->setText("Activating scene: " + sceneName);
        sceneList->setEnabled(false);
        activateSceneBtn->setEnabled(false);
        sceneActivationPending = true;
        queueLightWrite(false, [this, sceneName]() { sendScene(sceneName); });
    }

    void sendScene(const QString& sceneName) {
        QDBusPendingCall call = iface.asyncCall("ActivateScene", sceneName);
        QDBusPendingCallWatcher* watcher = new QDBusPendingCallWatcher(call, this);

        connect(watcher, &QDBusPendingCallWatcher::finished, this,
                [this, sceneName](QDBusPendingCallWatcher* w) {
                    sceneActivationPending = false;
                    QDBusPendingReply<QString> reply = *w;

                    if (reply.isError()) {
                        QString error = reply.error().message();
                        showWriteResult("Failed to activate scene");

                        // Provide user-friendly error messages
                        if (error.contains("unreachable") || error.contains("timeout") ||
                            error.contains("connection refused")) {
                            showStateError(bridgeUnreachable, "Bridge Unreachable",
                                           "Cannot connect to Hue Bridge.\n\n"
                                           "Check:\n"
                                           "• Bridge is powered on\n"
                                           "• Network connection is working\n"
                                           "• Bridge IP in config is correct");
                        } else if (error.contains("not found") || error.contains("unknown")) {
                            showErrorNotification(
                                "Scene Not Found",
                                "Scene '" + sceneName +
                                    "' could not be found.\n\n"
                                    "It may have been deleted. Refresh the scene list.",
                                KNotification::CloseOnTimeout);
                        } else {
                            showErrorNotification("Scene Activation Failed",
                                                  "Failed to activate: " + sceneName +
                                                      "\n\n"
                                                      "Error: " +
                                                      error +
                                                      "\n\n"
                                                      "Try again or check bridge status.",
                                                  KNotification::CloseOnTimeout);
                        }
                    } else {
                        QString result = reply.value();
                        activeScene = sceneName;
                        showWriteResult("Scene: " + sceneName);

                        // Show success notification with icon
                        KNotification* notif = new KNotification("sceneActivated");
                        notif->setTitle("Scene Activated");
                        notif->setText(sceneName);
                        notif->setIconName("preferences-desktop-display-color");
                        notif->setUrgency(KNotification::LowUrgency);
                        notif->sendEvent();
                    }

                    // Always refresh to update state and re-enable controls appropriately
                    QTimer::singleShot(reply.isError() ? 100 : 500, this,
                                       &HueControlDialog::refresh);
                    lightWriteFinished();

                    w->deleteLater();
                });
    }

    void onSyncToggled() {
        syncButton->setEnabled(false);
        syncTogglePending = true;

        QDBusPendingCall syncCall = iface.asyncCall("IsSyncing");
        QDBusPendingCall settingsCall = iface.asyncCall("GetSyncSettings");
        whenFinished(this, {syncCall, settingsCall}, [this, syncCall, settingsCall]() {
            QDBusReply<bool> syncReply = syncCall;
            if (syncReply.isValid() && syncReply.value()) {
                stopSync();
                return;
            }

            /**
             * A saved grant makes the portal skip its dialog, so there is
             * nothing for the user to answer. An unreadable reply means the
             * backend is not answering and the start is about to fail, so no
             * dialog is coming then either.
             */
            QDBusReply<QVariantMap> settingsReply = settingsCall;
            bool portalWillPrompt =
                settingsReply.isValid() && !settingsReply.value().value("hasScreenGrant").toBool();
            startSync(portalWillPrompt);
        });
    }

    void stopSync() {
        syncButton->setText("Stopping...");
        syncButton->setIcon(QIcon::fromTheme("process-stop"));

        QDBusPendingCall call = iface.asyncCall("StopSync");
        whenFinished(this, {call}, [this, call]() {
            syncTogglePending = false;
            stateWrites++;
            refreshIfStale();

            QDBusReply<bool> reply = call;
            if (reply.isValid() && reply.value()) {
                updateSyncButton(false);
                QTimer::singleShot(500, this, &HueControlDialog::refresh);

                // Show notification with action
                KNotification* notif = new KNotification("syncStopped");
                notif->setTitle("Screen Sync Stopped");
                notif->setText("Lights are no longer syncing with screen");
                notif->setIconName("preferences-desktop-display");
                notif->setUrgency(KNotification::LowUrgency);
                notif->sendEvent();
            } else {
                syncButton->setText("Stop Screen Sync");
                showErrorNotification("Failed to Stop Sync",
                                      reply.isValid() ? "Unknown error" : reply.error().message(),
                                      KNotification::CloseOnTimeout);
            }
            updateControlsEnabled();
        });
    }

    void startSync(bool portalWillPrompt) {
        syncButton->setText("Starting...");
        syncButton->setIcon(QIcon::fromTheme("chronometer"));
        statusLabel->setText(portalWillPrompt ? "Waiting for screen share approval"
                                              : "Starting screen sync...");

        if (portalWillPrompt) {
            KNotification* permNotif = new KNotification("syncPermission");
            permNotif->setTitle("Screen Sharing Permission Required");
            permNotif->setText(
                "Please select your monitor and click 'Share' in the dialog that appears.");
            permNotif->setIconName("dialog-information");
            permNotif->setUrgency(KNotification::LowUrgency);
            permNotif->sendEvent();
        }

        QDBusPendingCall call = iface.asyncCallWithTimeout("StartSync", startSyncTimeoutMs);
        QDBusPendingCallWatcher* watcher = new QDBusPendingCallWatcher(call, this);

        connect(
            watcher, &QDBusPendingCallWatcher::finished, this, [this](QDBusPendingCallWatcher* w) {
                syncTogglePending = false;
                updateControlsEnabled();
                stateWrites++;
                refreshIfStale();
                QDBusPendingReply<bool> reply = *w;

                if (reply.isError() || !reply.value()) {
                    QString error = reply.isValid() ? "Unknown error" : reply.error().message();
                    updateSyncButton(false);

                    // The refresh below leaves the label alone when the backend is gone
                    statusLabel->setText("Screen sync failed to start");

                    // Parse portal errors for user-friendly messages
                    if (error.contains("PortalError:permission_denied")) {
                        QString hint = error.section(':', 2);

                        KNotification* notif = new KNotification("syncFailed");
                        notif->setTitle("Screen Sharing Permission Denied");
                        notif->setText("You must approve the screen sharing dialog.\n\n" + hint +
                                       "\n\n"
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
                               error.contains("Entertainment") || error.contains("clientkey")) {
                        showErrorNotification("Screen Sync Not Configured",
                                              "Entertainment API is not configured.\n\n"
                                              "Setup required:\n"
                                              "1. Create Entertainment Area in Hue app\n"
                                              "2. Configure clientkey in the config file\n"
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
                                              "Error: " +
                                                  error,
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
                    notif->setText(QString("Lights are now syncing with your screen at %1 FPS")
                                       .arg(currentFps));
                    notif->setIconName("video-display");
                    notif->setUrgency(KNotification::LowUrgency);
                    notif->sendEvent();
                }

                QTimer::singleShot(500, this, &HueControlDialog::refresh);

                w->deleteLater();
            });
    }

    void onSettingsClicked() {
        if (!iface.isValid()) {
            QMessageBox::warning(this, "Service Unavailable", "Backend service is not running.");
            return;
        }

        roomPickerPending = true;
        settingsButton->setEnabled(false);

        // Get all scenes to extract room names
        QDBusPendingCall call = iface.asyncCallWithTimeout("GetScenes", getScenesTimeoutMs);
        whenFinished(this, {call}, [this, call]() {
            roomPickerPending = false;
            updateControlsEnabled();
            // The user may have closed the panel while the bridge was slow to answer
            if (!isVisible()) {
                return;
            }
            showRoomPicker(call);
        });
    }

    void showRoomPicker(const QDBusReply<QStringList>& scenesReply) {
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
            QMessageBox::information(
                this, "No Rooms",
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
            if (currentIndex < 0) {
                currentIndex = 0;
            }
        }

        QString selected = QInputDialog::getItem(this, "Select Room/Zone",
                                                 "Choose a room to filter scenes:", roomList,
                                                 currentIndex, false, &ok);

        if (ok && !selected.isEmpty()) {
            selectedRoom = selected;

            // Show confirmation once the refresh has applied, so its status text doesn't replace it
            statusAfterRefresh = (selected == "All Rooms")
                                     ? "Showing scenes from all rooms"
                                     : QString("Filtering scenes for: %1").arg(selected);

            refresh();
        }
    }

    /**
     * Checks bridge connection status and updates the UI accordingly.
     * @return true if the bridge just transitioned from disconnected to
     *   connected. Callers that don't already reload scenes/state themselves
     *   (pollStatus()) should call refresh() when this returns true.
     */
    bool checkConnectionStatus(const QDBusReply<QVariantMap>& reply) {
        if (!reply.isValid()) {
            // Method not available (old backend version) - assume connected
            return false;
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
                    .arg(lastError)
                    .arg(bridgeIP));
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
            return false;
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

                return true;
            }

            // Normal connected state
            updateConnectionState(CONNECTED);
            connectionDetailsLabel->hide();
        }

        return false;
    }

    void retryConnection() {
        if (!iface.isValid()) {
            statusLabel->setText("Backend not available");
            updateConnectionState(DISCONNECTED);
            showStateError(backendMissing, "Backend Not Running",
                           "KDE Hue Control backend could not be reached.\n\n"
                           "Start with: systemctl --user start hue-backend");
            return;
        }

        statusLabel->setText("Retrying connection...");
        updateConnectionState(CONNECTING);
        connectionDetailsLabel->hide();
        retryButton->setEnabled(false);

        QDBusPendingCall call = iface.asyncCall("RetryConnection");
        whenFinished(this, {call}, [this, call]() {
            retryButton->setEnabled(true);

            QDBusReply<bool> reply = call;
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

                showStateError(bridgeUnreachable, "Retry Failed",
                               "Bridge is still unreachable.\n\n"
                               "Check:\n"
                               "• Bridge is powered on\n"
                               "• Network connection is working\n"
                               "• Bridge IP in config is correct");
            }
        });
    }

    void updateGamingStatus(bool syncing, bool gamingActive) {
        // Update sync button and disable controls if syncing
        updateSyncButton(syncing);

        // Override button text for gaming mode
        if (syncing && gamingActive) {
            syncButton->setText("Stop Screen Sync");
            syncButton->setStyleSheet("QPushButton { color: #00ff00; font-weight: bold; }");
            statusLabel->setText(QString("Screen sync active  •  %1 FPS (Gaming)").arg(currentFps));
        } else {
            syncButton->setStyleSheet("");
        }
    }

    KNotification*
    showErrorNotification(const QString& title, const QString& message,
                          KNotification::NotificationFlags flags = KNotification::CloseOnTimeout) {
        KNotification* notif = new KNotification("error");
        notif->setTitle(title);
        notif->setText(message);
        notif->setIconName("dialog-error");
        notif->setUrgency(KNotification::NormalUrgency);
        notif->setFlags(flags);
        notif->sendEvent();
        return notif;
    }

    /**
     * Reports a condition that has an end rather than an event that happened.
     * The popup stays up until the condition clears, so the recovery path
     * needs the pointer to close it and a condition already on screen must not
     * be raised a second time. KNotification deletes itself when closed, by
     * the recovery path or by the user, which is what QPointer is tracking.
     */
    void showStateError(QPointer<KNotification>& tracked, const QString& title,
                        const QString& message) {
        if (tracked) {
            return;
        }
        tracked = showErrorNotification(title, message, KNotification::Persistent);
    }

    void clearStateError(QPointer<KNotification>& tracked) {
        if (!tracked) {
            return;
        }
        tracked->close();
        tracked.clear();
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
    QPushButton* activateSceneBtn;
    QPushButton* syncButton;
    QPushButton* retryButton;
    QPushButton* settingsButton;

    // Timers
    QTimer* brightnessTimer = nullptr;
    QTimer* connectionTimer;

    // State
    int pendingBrightness = 100;
    ConnectionState connectionState;
    int connectionRetryCount;
    QPointer<KNotification> backendMissing;
    QPointer<KNotification> bridgeUnreachable;
    QString selectedRoom;                // For room filtering
    QString activeScene;                 // Last successfully activated scene
    int currentFps = 30;                 // Configured sync FPS, refreshed from GetSyncSettings
    bool sceneActivationPending = false; // From a scene click until its ActivateScene returns
    bool powerChangePending = false;
    bool syncTogglePending = false;
    bool roomPickerPending = false;
    bool syncActive = false;
    int stateWrites = 0; // Finished calls that change light or sync state
    bool refreshPending = false;
    bool refreshQueued = false;
    bool refreshStale = false;
    QString statusAfterRefresh;
    bool pollPending = false;

    struct LightWrite {
        bool brightness;
        std::function<void()> start;
    };
    QList<LightWrite> lightWrites;
    bool lightWriteInFlight = false;

    HueBackend iface;
};

class HueTrayApp : public QApplication {
    Q_OBJECT

  public:
    HueTrayApp(int& argc, char** argv) : QApplication(argc, argv) {
        /**
         * KNotification looks the event ids up in a notifyrc named after the
         * application, so this has to match hue-tray.notifyrc.
         */
        setApplicationName("hue-tray");
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
        sni->setToolTip(idleIconName, "Hue Control", "Control Philips Hue lights");
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
        updateTooltip();           // Initial update
    }

  private slots:
    void showControlDialog() {
        if (controlDialog) {
            /**
             * show() triggers HueControlDialog::showEvent(), which already
             * calls refresh() - no need to call it again here.
             */
            controlDialog->show();
            controlDialog->raise();
            controlDialog->activateWindow();
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
        if (!iface.isValid()) {
            // Backend not running, keep defaults
            return;
        }

        QDBusPendingCall call = iface.asyncCall("GetTrayIcons");
        whenFinished(this, {call}, [this, call]() {
            QDBusMessage reply = call.reply();
            if (reply.type() == QDBusMessage::ErrorMessage) {
                // Method failed, keep defaults
                qDebug() << "Failed to get tray icons:" << reply.errorMessage();
                return;
            }

            /**
             * GetTrayIcons returns (string gaming, string syncing, string idle)
             * as three separate string arguments, not a variant or struct.
             */
            QList<QVariant> args = reply.arguments();
            if (args.size() >= 3) {
                gamingIconName = args[0].toString();
                syncingIconName = args[1].toString();
                idleIconName = args[2].toString();
                qDebug() << "Loaded icon names - Gaming:" << gamingIconName
                         << "Syncing:" << syncingIconName << "Idle:" << idleIconName;
                updateTooltip();
            }
        });
    }

    void updateTooltip() {
        if (!iface.isValid()) {
            sni->setToolTip(idleIconName, "Hue Control", "Backend not running");
            sni->setIconByName(idleIconName);
            return;
        }

        if (tooltipPending) {
            tooltipQueued = true;
            return;
        }
        tooltipPending = true;

        // Get gaming mode status
        QDBusPendingCall enabledCall = iface.asyncCall("IsGamingModeEnabled");
        QDBusPendingCall activeCall = iface.asyncCall("IsGamingModeActive");
        QDBusPendingCall syncCall = iface.asyncCall("IsSyncing");
        auto apply = [this, enabledCall, activeCall, syncCall]() {
            tooltipPending = false;

            QDBusReply<bool> enabledReply = enabledCall;
            bool gamingEnabled = enabledReply.isValid() && enabledReply.value();

            QDBusReply<bool> activeReply = activeCall;
            bool gamingActive = activeReply.isValid() && activeReply.value();

            QDBusReply<bool> syncReply = syncCall;
            bool syncing = syncReply.isValid() && syncReply.value();

            /**
             * Update icon based on gaming + sync state. Icons are cached member
             * variables loaded from backend config (not hardcoded), which allows
             * users to customize icons via the settings dialog.
             */
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

            if (tooltipQueued) {
                tooltipQueued = false;
                updateTooltip();
            }
        };
        whenFinished(this, {enabledCall, activeCall, syncCall}, apply);
    }

  private:
    KStatusNotifierItem* sni;
    HueControlDialog* controlDialog;

    // Cached icon names from backend config
    QString gamingIconName;
    QString syncingIconName;
    QString idleIconName;

    bool tooltipPending = false;
    bool tooltipQueued = false;
    HueBackend iface;
};

int main(int argc, char* argv[]) {
    HueTrayApp app(argc, argv);
    qDebug() << "Hue Control tray app starting...";
    return app.exec();
}

#include "main.moc"

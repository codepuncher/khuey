#include "settingsdialog.h"
#include <QDBusArgument>
#include <QDBusMessage>
#include <QDBusPendingReply>
#include <QDBusReply>
#include <QDebug>
#include <QGridLayout>
#include <QGroupBox>
#include <QHBoxLayout>
#include <QMessageBox>
#include <QVBoxLayout>
#include <QVariantMap>
#include <tuple>

SettingsDialog::SettingsDialog(QWidget* parent)
    : QDialog(parent), backend(new HueBackend(this)), currentFPS(30), currentSubsample(64) {
    setWindowTitle("Hue Control Settings");
    setMinimumSize(600, 500);

    setupUI();
    loadSettings();
}

void SettingsDialog::setupUI() {
    QVBoxLayout* mainLayout = new QVBoxLayout(this);

    // Tab widget
    tabWidget = new QTabWidget(this);

    // === Tab 1: Screen Sync ===
    QWidget* syncTab = new QWidget();
    QVBoxLayout* syncLayout = new QVBoxLayout(syncTab);

    // FPS control
    QGroupBox* fpsGroup = new QGroupBox("Frame Rate", syncTab);
    QGridLayout* fpsLayout = new QGridLayout(fpsGroup);

    fpsLayout->addWidget(new QLabel("FPS:"), 0, 0);
    fpsSlider = new QSlider(Qt::Horizontal, fpsGroup);
    fpsSlider->setRange(10, 60);
    fpsSlider->setValue(30);
    fpsLayout->addWidget(fpsSlider, 0, 1);

    fpsSpinBox = new QSpinBox(fpsGroup);
    fpsSpinBox->setRange(10, 60);
    fpsSpinBox->setValue(30);
    fpsLayout->addWidget(fpsSpinBox, 0, 2);

    connect(fpsSlider, &QSlider::valueChanged, fpsSpinBox, &QSpinBox::setValue);
    connect(fpsSpinBox, QOverload<int>::of(&QSpinBox::valueChanged), fpsSlider, &QSlider::setValue);

    QLabel* fpsHint = new QLabel("Higher FPS = smoother but more CPU usage", fpsGroup);
    fpsHint->setStyleSheet("QLabel { color: gray; font-size: 10pt; }");
    fpsLayout->addWidget(fpsHint, 1, 0, 1, 3);

    syncLayout->addWidget(fpsGroup);

    // Subsample width control
    QGroupBox* subsampleGroup = new QGroupBox("Processing Quality", syncTab);
    QGridLayout* subsampleLayout = new QGridLayout(subsampleGroup);

    subsampleLayout->addWidget(new QLabel("Subsample Width:"), 0, 0);
    subsampleSlider = new QSlider(Qt::Horizontal, subsampleGroup);
    subsampleSlider->setRange(16, 256);
    subsampleSlider->setValue(64);
    subsampleLayout->addWidget(subsampleSlider, 0, 1);

    subsampleSpinBox = new QSpinBox(subsampleGroup);
    subsampleSpinBox->setRange(16, 256);
    subsampleSpinBox->setValue(64);
    subsampleLayout->addWidget(subsampleSpinBox, 0, 2);

    connect(subsampleSlider, &QSlider::valueChanged, subsampleSpinBox, &QSpinBox::setValue);
    connect(subsampleSpinBox, QOverload<int>::of(&QSpinBox::valueChanged), subsampleSlider,
            &QSlider::setValue);

    QLabel* subsampleHint = new QLabel("Lower values = better performance, less color precision\n"
                                       "Applies the next time Screen Sync starts.",
                                       subsampleGroup);
    subsampleHint->setStyleSheet("QLabel { color: gray; font-size: 10pt; }");
    subsampleLayout->addWidget(subsampleHint, 1, 0, 1, 3);

    syncLayout->addWidget(subsampleGroup);

    // The screen-share portal takes no option naming an output, so the screen
    // is whatever was picked in its dialog. All this can do is drop the saved
    // grant so the dialog comes back.
    QGroupBox* captureGroup = new QGroupBox("Capture screen", syncTab);
    QVBoxLayout* captureLayout = new QVBoxLayout(captureGroup);

    resetCaptureButton = new QPushButton("Change capture screen...", captureGroup);
    connect(resetCaptureButton, &QPushButton::clicked, this, [this]() {
        readsInFlight++;
        updateInputState();

        QDBusPendingCall call = backend->asyncCall("ResetCaptureSource");
        whenFinished(this, {call}, [this, call]() {
            readsInFlight--;
            updateInputState();
            if (closing) {
                return;
            }

            QDBusReply<bool> reply = call;
            if (reply.isValid() && reply.value()) {
                QMessageBox::information(this, "Change capture screen",
                                         "The screen-share dialog will ask which screen to "
                                         "capture the next time Screen Sync starts.");
                return;
            }

            QString error =
                reply.isValid() ? "Failed to clear the saved screen" : reply.error().message();
            QMessageBox::warning(this, "Change capture screen", error);
        });
    });
    captureLayout->addWidget(resetCaptureButton);

    QLabel* captureHint = new QLabel("The screen is chosen in the system's screen-share dialog.\n"
                                     "This asks again the next time Screen Sync starts.",
                                     captureGroup);
    captureHint->setStyleSheet("QLabel { color: gray; font-size: 10pt; }");
    captureLayout->addWidget(captureHint);

    syncLayout->addWidget(captureGroup);

    // Gaming Mode checkbox
    QGroupBox* gamingGroup = new QGroupBox("Gaming Mode", syncTab);
    QVBoxLayout* gamingLayout = new QVBoxLayout(gamingGroup);

    gamingModeCheckbox = new QCheckBox("Automatically enable screen sync when gaming", gamingGroup);
    gamingModeCheckbox->setChecked(false);
    connect(gamingModeCheckbox, &QCheckBox::toggled, this, &SettingsDialog::onGamingModeToggled);
    gamingLayout->addWidget(gamingModeCheckbox);

    QLabel* gamingHint =
        new QLabel("When enabled, screen sync will automatically start when you play games.\n"
                   "Uses systemd-inhibit, power profile and Steam detection.",
                   gamingGroup);
    gamingHint->setWordWrap(true);
    gamingHint->setStyleSheet("QLabel { color: gray; font-size: 10pt; }");
    gamingLayout->addWidget(gamingHint);

    syncLayout->addWidget(gamingGroup);

    syncLayout->addStretch();
    tabWidget->addTab(syncTab, "Screen Sync");

    // === Tab 2: Light Control ===
    QWidget* lightTab = new QWidget();
    QVBoxLayout* lightLayout = new QVBoxLayout(lightTab);

    QGroupBox* roomGroup = new QGroupBox("Room/Zone Selection", lightTab);
    QVBoxLayout* roomLayout = new QVBoxLayout(roomGroup);

    QHBoxLayout* roomSelectLayout = new QHBoxLayout();
    roomSelectLayout->addWidget(new QLabel("Room/Zone:"));
    roomCombo = new QComboBox(roomGroup);
    roomSelectLayout->addWidget(roomCombo, 1);
    refreshRoomsButton = new QPushButton("Refresh", roomGroup);
    connect(refreshRoomsButton, &QPushButton::clicked, this,
            &SettingsDialog::onRefreshRoomsClicked);
    roomSelectLayout->addWidget(refreshRoomsButton);
    roomLayout->addLayout(roomSelectLayout);

    roomPreviewLabel =
        new QLabel("Select a room to control with power and brightness buttons", roomGroup);
    roomPreviewLabel->setWordWrap(true);
    roomPreviewLabel->setStyleSheet("QLabel { color: gray; padding: 8px; }");
    roomLayout->addWidget(roomPreviewLabel);

    lightLayout->addWidget(roomGroup);

    QGroupBox* startupSceneGroup = new QGroupBox("Login Behavior", lightTab);
    QVBoxLayout* startupSceneLayout = new QVBoxLayout(startupSceneGroup);

    QHBoxLayout* startupSceneSelectLayout = new QHBoxLayout();
    startupSceneSelectLayout->addWidget(new QLabel("Activate scene on login:"));
    startupSceneCombo = new QComboBox(startupSceneGroup);
    startupSceneCombo->addItem("(Disabled)", "");
    startupSceneSelectLayout->addWidget(startupSceneCombo, 1);
    startupSceneLayout->addLayout(startupSceneSelectLayout);

    QLabel* startupSceneHint =
        new QLabel("When set, this scene is activated automatically each time the backend starts "
                   "(e.g. on login).",
                   startupSceneGroup);
    startupSceneHint->setWordWrap(true);
    startupSceneHint->setStyleSheet("QLabel { color: gray; font-size: 10pt; }");
    startupSceneLayout->addWidget(startupSceneHint);

    lightLayout->addWidget(startupSceneGroup);
    lightLayout->addStretch();
    tabWidget->addTab(lightTab, "Light Control");

    // === Tab 3: Connection ===
    QWidget* connectionTab = new QWidget();
    QVBoxLayout* connectionLayout = new QVBoxLayout(connectionTab);

    QGroupBox* bridgeGroup = new QGroupBox("Bridge Connection", connectionTab);
    QGridLayout* bridgeLayout = new QGridLayout(bridgeGroup);

    bridgeLayout->addWidget(new QLabel("Bridge IP:"), 0, 0);
    bridgeIPEdit = new QLineEdit(bridgeGroup);
    bridgeIPEdit->setReadOnly(true); // For now, don't allow editing
    bridgeLayout->addWidget(bridgeIPEdit, 0, 1);

    bridgeLayout->addWidget(new QLabel("Status:"), 1, 0);
    connectionStatusLabel = new QLabel("Unknown", bridgeGroup);
    bridgeLayout->addWidget(connectionStatusLabel, 1, 1);

    QHBoxLayout* connectionButtonsLayout = new QHBoxLayout();
    testConnectionButton = new QPushButton("Test Connection", bridgeGroup);
    connect(testConnectionButton, &QPushButton::clicked, this,
            &SettingsDialog::onTestConnectionClicked);
    connectionButtonsLayout->addWidget(testConnectionButton);

    reconnectButton = new QPushButton("Reconnect", bridgeGroup);
    connect(reconnectButton, &QPushButton::clicked, this, [this]() {
        reconnectButton->setText("Reconnecting...");
        readsInFlight++;
        updateInputState();

        QDBusPendingCall call = backend->asyncCall("RetryConnection");
        whenFinished(this, {call}, [this, call]() {
            reconnectButton->setText("Reconnect");
            readsInFlight--;
            updateInputState();
            if (closing) {
                return;
            }

            QDBusReply<bool> reply = call;
            if (reply.isValid() && reply.value()) {
                QMessageBox::information(this, "Reconnect", "Successfully reconnected to bridge!");
                connectionStatusLabel->setText("Connected");
                connectionStatusLabel->setStyleSheet("QLabel { color: green; font-weight: bold; }");
                lastErrorLabel->clear();
                return;
            }

            QString error = reply.isValid() ? "Failed to reconnect" : reply.error().message();
            QMessageBox::warning(this, "Reconnect", error);
        });
    });
    connectionButtonsLayout->addWidget(reconnectButton);
    bridgeLayout->addLayout(connectionButtonsLayout, 2, 0, 1, 2);

    lastErrorLabel = new QLabel("", bridgeGroup);
    lastErrorLabel->setWordWrap(true);
    lastErrorLabel->setStyleSheet("QLabel { color: red; }");
    bridgeLayout->addWidget(lastErrorLabel, 3, 0, 1, 2);

    connectionHint =
        new QLabel("Tip: Edit the config file to change bridge IP or API key", bridgeGroup);
    connectionHint->setTextFormat(Qt::PlainText);
    connectionHint->setWordWrap(true);
    connectionHint->setStyleSheet("QLabel { color: gray; font-size: 10pt; margin-top: 8px; }");
    bridgeLayout->addWidget(connectionHint, 4, 0, 1, 2);

    connectionLayout->addWidget(bridgeGroup);
    connectionLayout->addStretch();
    tabWidget->addTab(connectionTab, "Connection");

    // === Tab 4: Appearance ===
    QWidget* appearanceTab = new QWidget();
    QVBoxLayout* appearanceLayout = new QVBoxLayout(appearanceTab);

    QGroupBox* iconGroup = new QGroupBox("Tray Icons", appearanceTab);
    QGridLayout* iconLayout = new QGridLayout(iconGroup);

    // Gaming Icon
    iconLayout->addWidget(new QLabel("Gaming + Sync:"), 0, 0);
    gamingIconButton = new KIconButton(iconGroup);
    gamingIconButton->setIcon("applications-games");
    gamingIconButton->setIconType(KIconLoader::NoGroup, KIconLoader::Any, false);
    gamingIconButton->setIconSize(32);
    gamingIconButton->setButtonIconSize(32);
    gamingIconButton->setToolTip("Click to choose icon for gaming mode + screen sync state");
    iconLayout->addWidget(gamingIconButton, 0, 1);

    gamingIconNameLabel = new QLabel("applications-games", iconGroup);
    gamingIconNameLabel->setStyleSheet("QLabel { color: gray; font-size: 9pt; }");
    iconLayout->addWidget(gamingIconNameLabel, 0, 2);

    connect(gamingIconButton, &KIconButton::iconChanged, this,
            [this](const QString& icon) { gamingIconNameLabel->setText(icon); });

    // Syncing Icon
    iconLayout->addWidget(new QLabel("Sync Active:"), 1, 0);
    syncingIconButton = new KIconButton(iconGroup);
    syncingIconButton->setIcon("media-record");
    syncingIconButton->setIconType(KIconLoader::NoGroup, KIconLoader::Any, false);
    syncingIconButton->setIconSize(32);
    syncingIconButton->setButtonIconSize(32);
    syncingIconButton->setToolTip("Click to choose icon for screen sync active state");
    iconLayout->addWidget(syncingIconButton, 1, 1);

    syncingIconNameLabel = new QLabel("media-record", iconGroup);
    syncingIconNameLabel->setStyleSheet("QLabel { color: gray; font-size: 9pt; }");
    iconLayout->addWidget(syncingIconNameLabel, 1, 2);

    connect(syncingIconButton, &KIconButton::iconChanged, this,
            [this](const QString& icon) { syncingIconNameLabel->setText(icon); });

    // Idle Icon
    iconLayout->addWidget(new QLabel("Idle:"), 2, 0);
    idleIconButton = new KIconButton(iconGroup);
    idleIconButton->setIcon("preferences-desktop-display-color");
    idleIconButton->setIconType(KIconLoader::NoGroup, KIconLoader::Any, false);
    idleIconButton->setIconSize(32);
    idleIconButton->setButtonIconSize(32);
    idleIconButton->setToolTip("Click to choose icon for idle state");
    iconLayout->addWidget(idleIconButton, 2, 1);

    idleIconNameLabel = new QLabel("preferences-desktop-display-color", iconGroup);
    idleIconNameLabel->setStyleSheet("QLabel { color: gray; font-size: 9pt; }");
    iconLayout->addWidget(idleIconNameLabel, 2, 2);

    connect(idleIconButton, &KIconButton::iconChanged, this,
            [this](const QString& icon) { idleIconNameLabel->setText(icon); });

    QLabel* iconHint = new QLabel("Click icon buttons to browse and choose from available icons\n"
                                  "Saved changes apply when this dialog closes.",
                                  iconGroup);
    iconHint->setWordWrap(true);
    iconHint->setStyleSheet("QLabel { color: gray; font-size: 10pt; margin-top: 8px; }");
    iconLayout->addWidget(iconHint, 3, 0, 1, 3);

    resetIconsButton = new QPushButton("Reset to Defaults", iconGroup);
    connect(resetIconsButton, &QPushButton::clicked, this, [this]() {
        gamingIconButton->setIcon("applications-games");
        syncingIconButton->setIcon("media-record");
        idleIconButton->setIcon("preferences-desktop-display-color");
        gamingIconNameLabel->setText("applications-games");
        syncingIconNameLabel->setText("media-record");
        idleIconNameLabel->setText("preferences-desktop-display-color");
        updateInputState();
    });
    iconLayout->addWidget(resetIconsButton, 4, 0, 1, 3);

    appearanceLayout->addWidget(iconGroup);
    appearanceLayout->addStretch();
    tabWidget->addTab(appearanceTab, "Appearance");

    mainLayout->addWidget(tabWidget);

    // Dialog buttons
    QHBoxLayout* buttonLayout = new QHBoxLayout();
    buttonLayout->addStretch();

    okButton = new QPushButton("OK", this);
    connect(okButton, &QPushButton::clicked, this, &SettingsDialog::onOkClicked);
    buttonLayout->addWidget(okButton);

    applyButton = new QPushButton("Apply", this);
    connect(applyButton, &QPushButton::clicked, this, &SettingsDialog::onApplyClicked);
    buttonLayout->addWidget(applyButton);

    cancelButton = new QPushButton("Cancel", this);
    connect(cancelButton, &QPushButton::clicked, this, &SettingsDialog::onCancelClicked);
    buttonLayout->addWidget(cancelButton);

    mainLayout->addLayout(buttonLayout);

    // Connected last: updateInputState reads every control and both buttons.
    const auto onFormChanged = [this]() { updateInputState(); };
    connect(fpsSpinBox, QOverload<int>::of(&QSpinBox::valueChanged), this, onFormChanged);
    connect(subsampleSpinBox, QOverload<int>::of(&QSpinBox::valueChanged), this, onFormChanged);
    connect(gamingModeCheckbox, &QCheckBox::toggled, this, onFormChanged);
    connect(roomCombo, QOverload<int>::of(&QComboBox::currentIndexChanged), this, onFormChanged);
    connect(startupSceneCombo, QOverload<int>::of(&QComboBox::currentIndexChanged), this,
            onFormChanged);
    connect(gamingIconButton, &KIconButton::iconChanged, this, onFormChanged);
    connect(syncingIconButton, &KIconButton::iconChanged, this, onFormChanged);
    connect(idleIconButton, &KIconButton::iconChanged, this, onFormChanged);
}

void SettingsDialog::loadSettings() {
    loading = true;
    updateInputState();

    QDBusPendingCall syncCall = backend->asyncCall("GetSyncSettings");
    QDBusPendingCall roomCall = backend->asyncCall("GetSelectedRoom");
    QDBusPendingCall gamingCall = backend->asyncCall("IsGamingModeEnabled");
    QDBusPendingCall startupSceneCall = backend->asyncCall("GetStartupScene");
    QDBusPendingCall scenesCall = backend->asyncCallWithTimeout("GetScenes", getScenesTimeoutMs);
    QDBusPendingCall bridgeCall = backend->asyncCall("GetBridgeSettings");
    QDBusPendingCall iconsCall = backend->asyncCall("GetTrayIcons");
    QDBusPendingCall roomsCall = backend->asyncCall("GetGroupedLights");

    auto apply = [this, syncCall, roomCall, gamingCall, startupSceneCall, scenesCall, bridgeCall,
                  iconsCall, roomsCall]() {
        QDBusReply<QVariantMap> syncReply = syncCall;
        if (syncReply.isValid()) {
            const QVariantMap settings = syncReply.value();
            currentFPS = settings["fps"].toInt();
            currentSubsample = settings["subsampleWidth"].toInt();

            fpsSlider->setValue(currentFPS);
            subsampleSlider->setValue(currentSubsample);
        }

        QDBusReply<QString> roomReply = roomCall;
        if (roomReply.isValid()) {
            currentRoomID = roomReply.value();
        }

        QDBusReply<bool> gamingReply = gamingCall;
        if (gamingReply.isValid()) {
            currentGamingMode = gamingReply.value();
            gamingModeCheckbox->setChecked(currentGamingMode);
        }

        // Selects currentRoomID, so it runs after the room reply is read.
        const QString roomsError = applyRooms(roomsCall, currentRoomID);

        QDBusReply<QString> startupSceneReply = startupSceneCall;
        QString currentStartupScene =
            startupSceneReply.isValid() ? startupSceneReply.value() : QString();

        QDBusReply<QStringList> scenesReply = scenesCall;
        for (const QString& scene : scenesReply.isValid() ? scenesReply.value() : QStringList()) {
            startupSceneCombo->addItem(scene, scene);
        }
        if (!currentStartupScene.isEmpty()) {
            int index = startupSceneCombo->findData(currentStartupScene);
            if (index < 0) {
                // Preserve it even if unresolved, so an unrelated Save can't clear it.
                startupSceneCombo->addItem(currentStartupScene, currentStartupScene);
                index = startupSceneCombo->count() - 1;
            }
            startupSceneCombo->setCurrentIndex(index);
        }

        QDBusReply<QVariantMap> bridgeReply = bridgeCall;
        if (bridgeReply.isValid()) {
            const QVariantMap settings = bridgeReply.value();
            bridgeIPEdit->setText(settings["bridgeIP"].toString());

            bool connected = settings["connected"].toBool();
            connectionStatusLabel->setText(connected ? "Connected" : "Disconnected");
            connectionStatusLabel->setStyleSheet(connected
                                                     ? "QLabel { color: green; font-weight: bold; }"
                                                     : "QLabel { color: red; font-weight: bold; }");

            QString lastError = settings["lastError"].toString();
            if (!lastError.isEmpty() && !connected) {
                lastErrorLabel->setText("Last error: " + lastError);
            }

            connectionHint->setText("Tip: Edit " +
                                    settings.value("configFile", "the config file").toString() +
                                    " to change bridge IP or API key");
        }

        QDBusPendingReply<QString, QString, QString> iconsReply = iconsCall;
        if (!iconsReply.isError()) {
            currentGamingIcon = iconsReply.argumentAt<0>();
            currentSyncingIcon = iconsReply.argumentAt<1>();
            currentIdleIcon = iconsReply.argumentAt<2>();

            gamingIconButton->setIcon(currentGamingIcon);
            syncingIconButton->setIcon(currentSyncingIcon);
            idleIconButton->setIcon(currentIdleIcon);

            gamingIconNameLabel->setText(currentGamingIcon);
            syncingIconNameLabel->setText(currentSyncingIcon);
            idleIconNameLabel->setText(currentIdleIcon);
        }

        /**
         * A failed read leaves a widget default the backend may not hold, so
         * the next save writes everything.
         */
        if (syncReply.isValid() && roomReply.isValid() && gamingReply.isValid() &&
            startupSceneReply.isValid() && !iconsReply.isError()) {
            savedValues = formValues();
            savedValues->roomID = currentRoomID;
        }
        loading = false;
        updateInputState();
        if (closing) {
            return;
        }

        /**
         * Warned last: the box blocks until dismissed, and the rest of the
         * dialog should be filled in and usable behind it.
         */
        if (!roomsError.isEmpty()) {
            QMessageBox::warning(this, "Error", roomsError);
        }
        if (!scenesReply.isValid()) {
            QMessageBox::warning(this, "Error",
                                 "Failed to load scenes: " + scenesReply.error().message());
        }
    };

    whenFinished(this,
                 {syncCall, roomCall, gamingCall, startupSceneCall, scenesCall, bridgeCall,
                  iconsCall, roomsCall},
                 apply);
}

/**
 * The dialog shows before its values arrive and its calls return to the event
 * loop, so the controls follow the work in flight: the tabs stay unusable
 * until values are in or written, and anything that starts another call is
 * disabled while one is outstanding.
 */
void SettingsDialog::updateInputState() {
    const bool busy = loading || saving || readsInFlight > 0;
    tabWidget->setEnabled(!loading && !saving);
    okButton->setEnabled(!busy);
    applyButton->setEnabled(!busy && !(formValues() == savedValues));
    /**
     * A second bridge call started under the first would report its result
     * over the newer one's.
     */
    testConnectionButton->setEnabled(!busy);
    reconnectButton->setEnabled(!busy);
    refreshRoomsButton->setEnabled(!busy);
    resetCaptureButton->setEnabled(!busy);
}

void SettingsDialog::reportSaveComplete(const std::function<void()>& onSaved) {
    QMessageBox::information(this, "Settings Saved",
                             "Settings saved successfully!\n\n"
                             "Subsample width applies the next time Screen Sync starts.\n"
                             "Icons change when this dialog closes.");
    onSaved();
}

void SettingsDialog::reject() {
    if (saving) {
        closePrompt = true;
        QMessageBox::StandardButton answer = QMessageBox::question(
            this, "Settings",
            "Settings are still being saved, and some have been saved already.\n\n"
            "Close anyway?",
            QMessageBox::Yes | QMessageBox::No, QMessageBox::No);
        closePrompt = false;
        if (answer != QMessageBox::Yes) {
            if (!pendingSaveError.isEmpty()) {
                QMessageBox::warning(this, "Settings Error", pendingSaveError);
                pendingSaveError.clear();
                return;
            }
            if (pendingSaveDone) {
                const std::function<void()> done = pendingSaveDone;
                pendingSaveDone = nullptr;
                reportSaveComplete(done);
            }
            return;
        }
    }

    closing = true;
    QDialog::reject();
}

bool SettingsDialog::FormValues::operator==(const FormValues& other) const {
    return std::tie(fps, subsample, gamingMode, roomID, startupScene, gamingIcon, syncingIcon,
                    idleIcon) == std::tie(other.fps, other.subsample, other.gamingMode,
                                          other.roomID, other.startupScene, other.gamingIcon,
                                          other.syncingIcon, other.idleIcon);
}

SettingsDialog::FormValues SettingsDialog::formValues() const {
    FormValues values;
    values.fps = fpsSpinBox->value();
    values.subsample = subsampleSpinBox->value();
    values.gamingMode = gamingModeCheckbox->isChecked();
    values.roomID = roomCombo->currentData().toString();
    if (values.roomID.isEmpty()) {
        // No rooms loaded, so the room read at load stands.
        values.roomID = currentRoomID;
    }
    values.startupScene = startupSceneCombo->currentData().toString();

    values.gamingIcon = gamingIconButton->icon();
    values.syncingIcon = syncingIconButton->icon();
    values.idleIcon = idleIconButton->icon();

    // Use defaults if empty
    if (values.gamingIcon.isEmpty()) {
        values.gamingIcon = "applications-games";
    }
    if (values.syncingIcon.isEmpty()) {
        values.syncingIcon = "media-record";
    }
    if (values.idleIcon.isEmpty()) {
        values.idleIcon = "preferences-desktop-display-color";
    }
    return values;
}

void SettingsDialog::saveSettings(const FormValues& values, std::function<void()> onSaved) {
    auto queue = std::make_shared<QList<Setter>>();
    queue->append({"SetSyncSettings", {values.fps, values.subsample}, "Screen Sync settings"});
    queue->append({"SetGamingMode", {values.gamingMode}, "gaming mode setting"});
    if (!values.roomID.isEmpty()) {
        queue->append({"SetSelectedRoom", {values.roomID}, "room selection"});
    }
    queue->append({"SetStartupScene", {values.startupScene}, "startup scene selection"});
    queue->append({"SetTrayIcons",
                   {values.gamingIcon, values.syncingIcon, values.idleIcon},
                   "tray icon settings"});

    saving = true;
    updateInputState();
    runSetters(queue, [this, values, onSaved]() {
        savedValues = values;
        updateInputState();
        onSaved();
    });
}

/**
 * Sends the queued setters one at a time, stopping at the first failure so a
 * later write can't land after an earlier one was rejected.
 */
void SettingsDialog::runSetters(std::shared_ptr<QList<Setter>> queue,
                                std::function<void()> onSaved) {
    if (queue->isEmpty()) {
        saving = false;
        updateInputState();
        if (closePrompt) {
            pendingSaveDone = onSaved;
            return;
        }
        if (closing) {
            return;
        }
        reportSaveComplete(onSaved);
        return;
    }

    const Setter setter = queue->takeFirst();
    QDBusPendingCall call = backend->asyncCallWithArgumentList(setter.method, setter.args);
    whenFinished(this, {call}, [this, call, setter, queue, onSaved]() {
        QDBusReply<bool> reply = call;
        if (!reply.isValid() || !reply.value()) {
            QString errorMsg = reply.isValid() ? "unknown error" : reply.error().message();
            savedValues.reset();
            saving = false;
            updateInputState();
            const QString message = "Failed to save " + setter.label + ": " + errorMsg;
            if (closePrompt) {
                pendingSaveError = message;
                return;
            }
            if (closing) {
                return;
            }
            QMessageBox::warning(this, "Settings Error", message);
            return;
        }
        if (setter.method == "SetSelectedRoom") {
            // formValues falls back to it, and a later setter can still fail.
            currentRoomID = setter.args.at(0).toString();
        }
        runSetters(queue, onSaved);
    });
}

bool SettingsDialog::validateSettings() {
    int fps = fpsSpinBox->value();
    int subsample = subsampleSpinBox->value();

    if (fps < 10 || fps > 60) {
        QMessageBox::warning(this, "Validation Error", "FPS must be between 10 and 60.");
        tabWidget->setCurrentIndex(0); // Switch to Screen Sync tab
        fpsSpinBox->setFocus();
        return false;
    }

    if (subsample < 16 || subsample > 256) {
        QMessageBox::warning(this, "Validation Error",
                             "Subsample width must be between 16 and 256.");
        tabWidget->setCurrentIndex(0); // Switch to Screen Sync tab
        subsampleSpinBox->setFocus();
        return false;
    }

    return true;
}

void SettingsDialog::onApplyClicked() {
    const FormValues values = formValues();
    if (values == savedValues || !validateSettings()) {
        return;
    }
    saveSettings(values, []() {});
}

void SettingsDialog::onOkClicked() {
    const FormValues values = formValues();
    if (values == savedValues) {
        accept();
        return;
    }
    if (!validateSettings()) {
        return;
    }
    saveSettings(values, [this]() { accept(); });
}

void SettingsDialog::onCancelClicked() { reject(); }

void SettingsDialog::onTestConnectionClicked() {
    testConnectionButton->setText("Testing...");
    readsInFlight++;
    updateInputState();

    QDBusPendingCall call = backend->asyncCall("TestBridgeConnection");
    whenFinished(this, {call}, [this, call]() {
        testConnectionButton->setText("Test Connection");
        readsInFlight--;
        updateInputState();
        if (closing) {
            return;
        }

        QDBusReply<bool> reply = call;
        if (reply.isValid() && reply.value()) {
            QMessageBox::information(this, "Connection Test", "Bridge is reachable!");
            connectionStatusLabel->setText("Connected");
            connectionStatusLabel->setStyleSheet("QLabel { color: green; font-weight: bold; }");
            lastErrorLabel->clear();
            return;
        }

        QString error = reply.isValid() ? "Bridge is unreachable" : reply.error().message();
        QMessageBox::warning(this, "Connection Test", error);
        connectionStatusLabel->setText("Disconnected");
        connectionStatusLabel->setStyleSheet("QLabel { color: red; font-weight: bold; }");
        lastErrorLabel->setText("Error: " + error);
    });
}

void SettingsDialog::onRefreshRoomsClicked() {
    refreshRoomsButton->setText("Loading...");
    readsInFlight++;
    updateInputState();

    QDBusPendingCall call = backend->asyncCall("GetGroupedLights");
    whenFinished(this, {call}, [this, call]() {
        readsInFlight--;
        updateInputState();
        if (closing) {
            return;
        }

        /**
         * Read when the reply lands, not when Refresh was clicked: the combo
         * stays usable during the call, and saveSettings reads whatever it
         * holds.
         */
        const QString selected = roomCombo->currentData().toString();
        const QString error = applyRooms(call, selected.isEmpty() ? currentRoomID : selected);
        if (!error.isEmpty()) {
            QMessageBox::warning(this, "Error", error);
        }
    });
}

/**
 * Fills the room combo from a GetGroupedLights reply and returns the message to
 * warn with, empty when there is nothing to report. The combo is only cleared
 * once the reply is known good, so a failed refresh keeps the current items and
 * the selection saveSettings reads.
 */
QString SettingsDialog::applyRooms(const QDBusPendingCall& call, const QString& selectRoomID) {
    refreshRoomsButton->setText("Refresh");

    QDBusMessage reply = call.reply();

    /**
     * Leaves an empty combo with something to show, without wiping items a
     * failed refresh should keep.
     */
    auto fail = [this](const QString& preview, const QString& message) {
        roomPreviewLabel->setText(preview);
        if (roomCombo->count() == 0) {
            roomCombo->addItem("No rooms loaded", "");
        }
        return message;
    };

    if (reply.type() == QDBusMessage::ErrorMessage) {
        return fail("Failed to load rooms from bridge",
                    "Failed to load rooms: " + reply.errorMessage());
    }

    if (reply.arguments().isEmpty()) {
        return fail("No data from bridge", "The backend returned no rooms.");
    }

    // Extract the QDBusArgument from the message - MUST be const!
    QVariant var = reply.arguments().at(0);
    if (!var.canConvert<QDBusArgument>()) {
        return fail("Invalid response from bridge",
                    "The backend returned rooms in an unexpected format.");
    }

    roomCombo->clear();

    const QDBusArgument arg = var.value<QDBusArgument>();
    arg.beginArray();

    while (!arg.atEnd()) {
        arg.beginStructure();
        QString id, name, type;
        arg >> id >> name >> type;
        arg.endStructure();

        roomCombo->addItem(QString("%1 (%2)").arg(name, type), id);

        // Select current room
        if (id == selectRoomID) {
            roomCombo->setCurrentIndex(roomCombo->count() - 1);
        }
    }
    arg.endArray();

    if (roomCombo->count() == 0) {
        roomCombo->addItem("No rooms found", "");
        roomPreviewLabel->setText("No rooms or zones available. Check bridge connection.");
        return QString();
    }
    roomPreviewLabel->setText(QString("Found %1 room(s)/zone(s)").arg(roomCombo->count()));
    return QString();
}

void SettingsDialog::onFpsChanged(int value) {
    // Real-time validation feedback
    if (value < 10 || value > 60) {
        fpsSpinBox->setStyleSheet("QSpinBox { background-color: #ffe6e6; }");
    } else {
        fpsSpinBox->setStyleSheet("");
    }
}

void SettingsDialog::onSubsampleChanged(int value) {
    // Real-time validation feedback
    if (value < 16 || value > 256) {
        subsampleSpinBox->setStyleSheet("QSpinBox { background-color: #ffe6e6; }");
    } else {
        subsampleSpinBox->setStyleSheet("");
    }
}

void SettingsDialog::onGamingModeToggled(bool checked) {
    // Saved on Apply/OK, not immediately.
    qDebug() << "Gaming mode toggled:" << (checked ? "enabled" : "disabled");
}

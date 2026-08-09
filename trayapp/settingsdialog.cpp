#include "settingsdialog.h"
#include <QDBusArgument>
#include <QDBusMessage>
#include <QDBusReply>
#include <QDebug>
#include <QGridLayout>
#include <QGroupBox>
#include <QHBoxLayout>
#include <QMessageBox>
#include <QVBoxLayout>
#include <QVariantMap>

SettingsDialog::SettingsDialog(QWidget* parent)
    : QDialog(parent),
      dbusInterface(new QDBusInterface("org.kde.plasma.hue", "/org/kde/plasma/hue",
                                       "org.kde.plasma.hue", QDBusConnection::sessionBus(), this)),
      currentFPS(30), currentSubsample(64) {
    setWindowTitle("Hue Control Settings");
    setMinimumSize(600, 500);

    setupUI();
    loadSettings();
}

SettingsDialog::~SettingsDialog() { delete dbusInterface; }

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

    QLabel* subsampleHint =
        new QLabel("Lower values = better performance, less color precision", subsampleGroup);
    subsampleHint->setStyleSheet("QLabel { color: gray; font-size: 10pt; }");
    subsampleLayout->addWidget(subsampleHint, 1, 0, 1, 3);

    syncLayout->addWidget(subsampleGroup);

    // Monitor selection
    QGroupBox* monitorGroup = new QGroupBox("Monitor", syncTab);
    QHBoxLayout* monitorLayout = new QHBoxLayout(monitorGroup);
    monitorLayout->addWidget(new QLabel("Monitor:"));
    monitorCombo = new QComboBox(monitorGroup);
    monitorCombo->addItem("Default (Primary Monitor)", "");
    monitorLayout->addWidget(monitorCombo, 1);
    syncLayout->addWidget(monitorGroup);

    // Gaming Mode checkbox
    QGroupBox* gamingGroup = new QGroupBox("Gaming Mode", syncTab);
    QVBoxLayout* gamingLayout = new QVBoxLayout(gamingGroup);

    gamingModeCheckbox = new QCheckBox("Automatically enable screen sync when gaming", gamingGroup);
    gamingModeCheckbox->setChecked(false);
    connect(gamingModeCheckbox, &QCheckBox::toggled, this, &SettingsDialog::onGamingModeToggled);
    gamingLayout->addWidget(gamingModeCheckbox);

    QLabel* gamingHint =
        new QLabel("When enabled, screen sync will automatically start when you play games.\n"
                   "Uses GameMode and fullscreen window detection.",
                   gamingGroup);
    gamingHint->setWordWrap(true);
    gamingHint->setStyleSheet("QLabel { color: gray; font-size: 10pt; }");
    gamingLayout->addWidget(gamingHint);

    syncLayout->addWidget(gamingGroup);

    // Info label
    QLabel* infoLabel =
        new QLabel("ℹ️  Note: Screen Sync must be restarted for changes to take effect.", syncTab);
    infoLabel->setWordWrap(true);
    infoLabel->setStyleSheet(
        "QLabel { color: #3584e4; background-color: #e5f2ff; padding: 8px; border-radius: 4px; }");
    syncLayout->addWidget(infoLabel);

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
        QDBusReply<bool> reply = dbusInterface->call("RetryConnection");
        if (reply.isValid() && reply.value()) {
            QMessageBox::information(this, "Reconnect", "✓ Successfully reconnected to bridge!");
            connectionStatusLabel->setText("✓ Connected");
            connectionStatusLabel->setStyleSheet("QLabel { color: green; font-weight: bold; }");
            lastErrorLabel->clear();
        } else {
            QString error = reply.isValid() ? "Failed to reconnect" : reply.error().message();
            QMessageBox::warning(this, "Reconnect", "✗ " + error);
        }
    });
    connectionButtonsLayout->addWidget(reconnectButton);
    bridgeLayout->addLayout(connectionButtonsLayout, 2, 0, 1, 2);

    lastErrorLabel = new QLabel("", bridgeGroup);
    lastErrorLabel->setWordWrap(true);
    lastErrorLabel->setStyleSheet("QLabel { color: red; }");
    bridgeLayout->addWidget(lastErrorLabel, 3, 0, 1, 2);

    QLabel* connectionHint = new QLabel(
        "💡 Tip: Edit ~/.openhue/config.yaml to change bridge IP or API key", bridgeGroup);
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

    QLabel* iconHint =
        new QLabel("ℹ️  Click icon buttons to browse and choose from available icons\n"
                   "Changes apply after restarting the tray app.",
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
}

void SettingsDialog::loadSettings() {
    // Load Screen Sync settings
    QDBusMessage syncReply = dbusInterface->call("GetSyncSettings");
    if (syncReply.type() != QDBusMessage::ErrorMessage && !syncReply.arguments().isEmpty()) {
        // Extract variant map from DBus reply
        QVariant var = syncReply.arguments().at(0);

        // DBus a{sv} comes as QDBusArgument, need to properly extract it
        if (var.canConvert<QDBusArgument>()) {
            QVariantMap settings;
            const QDBusArgument arg = var.value<QDBusArgument>();
            arg.beginMap();
            while (!arg.atEnd()) {
                QString key;
                QVariant value;
                arg.beginMapEntry();
                arg >> key >> value;
                arg.endMapEntry();
                settings[key] = value;
            }
            arg.endMap();

            currentFPS = settings["fps"].toInt();
            currentSubsample = settings["subsampleWidth"].toInt();
            currentMonitor = settings["monitor"].toString();

            fpsSlider->setValue(currentFPS);
            subsampleSlider->setValue(currentSubsample);

            // Set monitor if specified
            if (!currentMonitor.isEmpty()) {
                int index = monitorCombo->findData(currentMonitor);
                if (index >= 0) {
                    monitorCombo->setCurrentIndex(index);
                }
            }
        }
    }

    // Load room selection
    QDBusReply<QString> roomReply = dbusInterface->call("GetSelectedRoom");
    if (roomReply.isValid()) {
        currentRoomID = roomReply.value();
    }

    // Load gaming mode setting
    QDBusReply<bool> gamingReply = dbusInterface->call("IsGamingModeEnabled");
    if (gamingReply.isValid()) {
        currentGamingMode = gamingReply.value();
        gamingModeCheckbox->setChecked(currentGamingMode);
    }

    // Auto-load rooms list on dialog open (fixed QDBusArgument extraction)
    onRefreshRoomsClicked();

    QDBusReply<QString> startupSceneReply = dbusInterface->call("GetStartupScene");
    QString currentStartupScene =
        startupSceneReply.isValid() ? startupSceneReply.value() : QString();

    QDBusReply<QStringList> scenesReply = dbusInterface->call("GetScenes");
    if (scenesReply.isValid()) {
        for (const QString& scene : scenesReply.value()) {
            startupSceneCombo->addItem(scene, scene);
        }
    } else {
        QMessageBox::warning(this, "Error",
                             "Failed to load scenes: " + scenesReply.error().message());
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

    // Load bridge settings
    QDBusMessage bridgeReply = dbusInterface->call("GetBridgeSettings");
    if (bridgeReply.type() != QDBusMessage::ErrorMessage && !bridgeReply.arguments().isEmpty()) {
        // Extract variant map from DBus reply
        QVariant var = bridgeReply.arguments().at(0);

        // DBus a{sv} comes as QDBusArgument, need to properly extract it
        if (var.canConvert<QDBusArgument>()) {
            QVariantMap settings;
            const QDBusArgument arg = var.value<QDBusArgument>();
            arg.beginMap();
            while (!arg.atEnd()) {
                QString key;
                QVariant value;
                arg.beginMapEntry();
                arg >> key >> value;
                arg.endMapEntry();
                settings[key] = value;
            }
            arg.endMap();

            bridgeIPEdit->setText(settings["bridgeIP"].toString());

            bool connected = settings["connected"].toBool();
            connectionStatusLabel->setText(connected ? "✓ Connected" : "✗ Disconnected");
            connectionStatusLabel->setStyleSheet(connected
                                                     ? "QLabel { color: green; font-weight: bold; }"
                                                     : "QLabel { color: red; font-weight: bold; }");

            QString lastError = settings["lastError"].toString();
            if (!lastError.isEmpty() && !connected) {
                lastErrorLabel->setText("Last error: " + lastError);
            }
        }
    }

    // Load tray icon settings
    QDBusReply<QString> gamingIconReply = dbusInterface->call("GetTrayIcons");
    if (gamingIconReply.isValid()) {
        QDBusMessage iconReply = dbusInterface->call("GetTrayIcons");
        if (iconReply.type() != QDBusMessage::ErrorMessage && iconReply.arguments().size() >= 3) {
            currentGamingIcon = iconReply.arguments().at(0).toString();
            currentSyncingIcon = iconReply.arguments().at(1).toString();
            currentIdleIcon = iconReply.arguments().at(2).toString();

            gamingIconButton->setIcon(currentGamingIcon);
            syncingIconButton->setIcon(currentSyncingIcon);
            idleIconButton->setIcon(currentIdleIcon);

            gamingIconNameLabel->setText(currentGamingIcon);
            syncingIconNameLabel->setText(currentSyncingIcon);
            idleIconNameLabel->setText(currentIdleIcon);
        }
    }
}

void SettingsDialog::saveSettings() {
    auto callSetter = [this](const QString& method, const QVariantList& args,
                             const QString& label) -> bool {
        QDBusReply<bool> reply = dbusInterface->callWithArgumentList(QDBus::Block, method, args);
        if (!reply.isValid() || !reply.value()) {
            QString errorMsg = reply.isValid() ? "unknown error" : reply.error().message();
            QMessageBox::warning(this, "Settings Error",
                                 "Failed to save " + label + ": " + errorMsg);
            return false;
        }
        return true;
    };

    int fps = fpsSpinBox->value();
    int subsample = subsampleSpinBox->value();
    QString monitor = monitorCombo->currentData().toString();
    if (!callSetter("SetSyncSettings", {fps, subsample, monitor}, "Screen Sync settings")) {
        return;
    }

    bool gamingMode = gamingModeCheckbox->isChecked();
    if (!callSetter("SetGamingMode", {gamingMode}, "gaming mode setting")) {
        return;
    }

    QString roomID = roomCombo->currentData().toString();
    if (!roomID.isEmpty() && !callSetter("SetSelectedRoom", {roomID}, "room selection")) {
        return;
    }

    QString startupScene = startupSceneCombo->currentData().toString();
    if (!callSetter("SetStartupScene", {startupScene}, "startup scene selection")) {
        return;
    }

    QString gamingIcon = gamingIconButton->icon();
    QString syncingIcon = syncingIconButton->icon();
    QString idleIcon = idleIconButton->icon();

    // Use defaults if empty
    if (gamingIcon.isEmpty())
        gamingIcon = "applications-games";
    if (syncingIcon.isEmpty())
        syncingIcon = "media-record";
    if (idleIcon.isEmpty())
        idleIcon = "preferences-desktop-display-color";

    if (!callSetter("SetTrayIcons", {gamingIcon, syncingIcon, idleIcon}, "tray icon settings")) {
        return;
    }

    QMessageBox::information(
        this, "Settings Saved",
        "Settings saved successfully!\n\n"
        "Note: Restart the tray app for icon changes to take effect.\n"
        "If Screen Sync is running, restart it for FPS/quality changes to take effect.");
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
    if (validateSettings()) {
        saveSettings();
    }
}

void SettingsDialog::onOkClicked() {
    if (validateSettings()) {
        saveSettings();
        accept();
    }
}

void SettingsDialog::onCancelClicked() { reject(); }

void SettingsDialog::onTestConnectionClicked() {
    testConnectionButton->setEnabled(false);
    testConnectionButton->setText("Testing...");

    QDBusReply<bool> reply = dbusInterface->call("TestBridgeConnection");

    testConnectionButton->setEnabled(true);
    testConnectionButton->setText("Test Connection");

    if (reply.isValid() && reply.value()) {
        QMessageBox::information(this, "Connection Test", "✓ Bridge is reachable!");
        connectionStatusLabel->setText("✓ Connected");
        connectionStatusLabel->setStyleSheet("QLabel { color: green; font-weight: bold; }");
        lastErrorLabel->clear();
    } else {
        QString error = reply.isValid() ? "Bridge is unreachable" : reply.error().message();
        QMessageBox::warning(this, "Connection Test", "✗ " + error);
        connectionStatusLabel->setText("✗ Disconnected");
        connectionStatusLabel->setStyleSheet("QLabel { color: red; font-weight: bold; }");
        lastErrorLabel->setText("Error: " + error);
    }
}

void SettingsDialog::onRefreshRoomsClicked() {
    roomCombo->clear();
    refreshRoomsButton->setEnabled(false);
    refreshRoomsButton->setText("Loading...");

    // Get grouped lights from backend using QDBusMessage
    QDBusMessage reply = dbusInterface->call("GetGroupedLights");

    refreshRoomsButton->setEnabled(true);
    refreshRoomsButton->setText("Refresh");

    if (reply.type() == QDBusMessage::ErrorMessage) {
        QMessageBox::warning(this, "Error", "Failed to load rooms: " + reply.errorMessage());
        roomCombo->addItem("Error loading rooms", "");
        roomPreviewLabel->setText("❌ Failed to load rooms from bridge");
        return;
    }

    if (reply.arguments().isEmpty()) {
        roomCombo->addItem("No data received", "");
        roomPreviewLabel->setText("⚠️  No data from bridge");
        return;
    }

    // Extract the QDBusArgument from the message - MUST be const!
    QVariant var = reply.arguments().at(0);
    if (!var.canConvert<QDBusArgument>()) {
        roomCombo->addItem("Invalid data format", "");
        roomPreviewLabel->setText("❌ Invalid response from bridge");
        return;
    }

    const QDBusArgument arg = var.value<QDBusArgument>();
    arg.beginArray();

    int count = 0;
    while (!arg.atEnd()) {
        arg.beginStructure();
        QString id, name, type;
        arg >> id >> name >> type;
        arg.endStructure();

        roomCombo->addItem(QString("%1 (%2)").arg(name, type), id);

        // Select current room
        if (id == currentRoomID) {
            roomCombo->setCurrentIndex(roomCombo->count() - 1);
        }

        count++;
    }
    arg.endArray();

    if (roomCombo->count() == 0) {
        roomCombo->addItem("No rooms found", "");
        roomPreviewLabel->setText("⚠️  No rooms or zones available. Check bridge connection.");
    } else {
        roomPreviewLabel->setText(QString("Found %1 room(s)/zone(s)").arg(roomCombo->count()));
    }
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

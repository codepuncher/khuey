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
    qDebug() << "Loading settings from DBus...";

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

            qDebug() << "Sync settings loaded:" << settings;

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
    } else if (syncReply.type() == QDBusMessage::ErrorMessage) {
        qDebug() << "Error loading sync settings:" << syncReply.errorMessage();
    }

    // Load room selection
    QDBusReply<QString> roomReply = dbusInterface->call("GetSelectedRoom");
    if (roomReply.isValid()) {
        currentRoomID = roomReply.value();
        qDebug() << "Selected room loaded:" << currentRoomID;
    } else {
        qDebug() << "Error loading selected room:" << roomReply.error().message();
    }

    // Auto-load rooms list on dialog open (fixed QDBusArgument extraction)
    onRefreshRoomsClicked();

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

            qDebug() << "Bridge settings loaded:" << settings;

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
    } else if (bridgeReply.type() == QDBusMessage::ErrorMessage) {
        qDebug() << "Error loading bridge settings:" << bridgeReply.errorMessage();
    }

    qDebug() << "Settings loading complete";
}

void SettingsDialog::saveSettings() {
    // Save Screen Sync settings
    int fps = fpsSpinBox->value();
    int subsample = subsampleSpinBox->value();
    QString monitor = monitorCombo->currentData().toString();

    QDBusReply<bool> syncReply = dbusInterface->call("SetSyncSettings", fps, subsample, monitor);
    if (!syncReply.isValid() || !syncReply.value()) {
        QString errorMsg =
            syncReply.isValid() ? "Failed to save settings" : syncReply.error().message();
        QMessageBox::warning(this, "Settings Error",
                             "Failed to save Screen Sync settings: " + errorMsg);
        return;
    }

    // Save room selection
    QString roomID = roomCombo->currentData().toString();
    if (!roomID.isEmpty()) {
        QDBusReply<bool> roomReply = dbusInterface->call("SetSelectedRoom", roomID);
        if (!roomReply.isValid() || !roomReply.value()) {
            QString errorMsg =
                roomReply.isValid() ? "Failed to save room" : roomReply.error().message();
            QMessageBox::warning(this, "Settings Error",
                                 "Failed to save room selection: " + errorMsg);
            return;
        }
    }

    QMessageBox::information(
        this, "Settings Saved",
        "Settings saved successfully!\n\n"
        "Note: If Screen Sync is running, restart it for changes to take effect.");
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
    qDebug() << "onRefreshRoomsClicked called";
    roomCombo->clear();
    refreshRoomsButton->setEnabled(false);
    refreshRoomsButton->setText("Loading...");

    // Get grouped lights from backend using QDBusMessage
    QDBusMessage reply = dbusInterface->call("GetGroupedLights");

    refreshRoomsButton->setEnabled(true);
    refreshRoomsButton->setText("Refresh");

    if (reply.type() == QDBusMessage::ErrorMessage) {
        qDebug() << "GetGroupedLights failed:" << reply.errorMessage();
        QMessageBox::warning(this, "Error", "Failed to load rooms: " + reply.errorMessage());
        roomCombo->addItem("Error loading rooms", "");
        roomPreviewLabel->setText("❌ Failed to load rooms from bridge");
        return;
    }

    if (reply.arguments().isEmpty()) {
        qDebug() << "GetGroupedLights returned no arguments";
        roomCombo->addItem("No data received", "");
        roomPreviewLabel->setText("⚠️  No data from bridge");
        return;
    }

    qDebug() << "GetGroupedLights replied successfully";

    // Extract the QDBusArgument from the message - MUST be const!
    QVariant var = reply.arguments().at(0);
    if (!var.canConvert<QDBusArgument>()) {
        qDebug() << "Cannot convert reply to QDBusArgument";
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

        qDebug() << "  Room/Zone:" << name << "(" << type << ") ID:" << id;
        roomCombo->addItem(QString("%1 (%2)").arg(name, type), id);

        // Select current room
        if (id == currentRoomID) {
            roomCombo->setCurrentIndex(roomCombo->count() - 1);
        }

        count++;
    }
    arg.endArray();

    qDebug() << "Loaded" << count << "room(s)/zone(s)";

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

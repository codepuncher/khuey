#ifndef SETTINGSDIALOG_H
#define SETTINGSDIALOG_H

#include <QCheckBox>
#include <QComboBox>
#include <QDBusInterface>
#include <QDialog>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QSlider>
#include <QSpinBox>
#include <QTabWidget>
#include <KIconButton>

class SettingsDialog : public QDialog {
    Q_OBJECT

  public:
    explicit SettingsDialog(QWidget* parent = nullptr);
    ~SettingsDialog();

  private slots:
    void onApplyClicked();
    void onOkClicked();
    void onCancelClicked();
    void onTestConnectionClicked();
    void onRefreshRoomsClicked();
    void onFpsChanged(int value);
    void onSubsampleChanged(int value);
    void onGamingModeToggled(bool checked);

  private:
    void setupUI();
    void loadSettings();
    void saveSettings();
    bool validateSettings();

    // UI Components
    QTabWidget* tabWidget;

    // Screen Sync tab
    QSlider* fpsSlider;
    QSpinBox* fpsSpinBox;
    QSlider* subsampleSlider;
    QSpinBox* subsampleSpinBox;
    QComboBox* monitorCombo;
    QLabel* syncStatusLabel;
    QCheckBox* gamingModeCheckbox;

    // Light Control tab
    QComboBox* roomCombo;
    QLabel* roomPreviewLabel;
    QPushButton* refreshRoomsButton;

    // Connection tab
    QLineEdit* bridgeIPEdit;
    QLabel* connectionStatusLabel;
    QPushButton* testConnectionButton;
    QPushButton* reconnectButton;
    QLabel* lastErrorLabel;

    // Appearance tab
    KIconButton* gamingIconButton;
    KIconButton* syncingIconButton;
    KIconButton* idleIconButton;
    QLabel* gamingIconNameLabel;
    QLabel* syncingIconNameLabel;
    QLabel* idleIconNameLabel;
    QPushButton* resetIconsButton;

    // Dialog buttons
    QPushButton* okButton;
    QPushButton* applyButton;
    QPushButton* cancelButton;

    // DBus interface
    QDBusInterface* dbusInterface;

    // Current values
    int currentFPS;
    int currentSubsample;
    QString currentMonitor;
    QString currentRoomID;
    bool currentGamingMode;
    QString currentGamingIcon;
    QString currentSyncingIcon;
    QString currentIdleIcon;
};

#endif // SETTINGSDIALOG_H

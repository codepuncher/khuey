#ifndef SETTINGSDIALOG_H
#define SETTINGSDIALOG_H

#include "huebackend.h"
#include <KIconButton>
#include <QCheckBox>
#include <QComboBox>
#include <QDBusPendingCall>
#include <QDialog>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QSlider>
#include <QSpinBox>
#include <QTabWidget>
#include <QVariantList>
#include <functional>
#include <memory>
#include <optional>

class SettingsDialog : public QDialog {
    Q_OBJECT

  public:
    explicit SettingsDialog(QWidget* parent = nullptr);

    void reject() override;

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
    QString applyRooms(const QDBusPendingCall& call, const QString& selectRoomID);

    struct Setter {
        QString method;
        QVariantList args;
        QString label;
    };
    struct FormValues {
        int fps = 0;
        int subsample = 0;
        bool gamingMode = false;
        QString roomID;
        QString startupScene;
        QString gamingIcon;
        QString syncingIcon;
        QString idleIcon;

        bool operator==(const FormValues& other) const;
    };
    FormValues formValues() const;
    void saveSettings(const FormValues& values, std::function<void()> onSaved);
    void reportSaveComplete(const std::function<void()>& onSaved);
    void runSetters(std::shared_ptr<QList<Setter>> queue, std::function<void()> onSaved);
    bool validateSettings();
    void updateInputState();

    // UI Components
    QTabWidget* tabWidget;

    // Screen Sync tab
    QSlider* fpsSlider;
    QSpinBox* fpsSpinBox;
    QSlider* subsampleSlider;
    QSpinBox* subsampleSpinBox;
    QPushButton* resetCaptureButton;
    QLabel* syncStatusLabel;
    QCheckBox* gamingModeCheckbox;

    // Light Control tab
    QComboBox* roomCombo;
    QLabel* roomPreviewLabel;
    QPushButton* refreshRoomsButton;
    QComboBox* startupSceneCombo;

    // Connection tab
    QLineEdit* bridgeIPEdit;
    QLabel* connectionStatusLabel;
    QPushButton* testConnectionButton;
    QPushButton* reconnectButton;
    QLabel* lastErrorLabel;
    QLabel* connectionHint;

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
    HueBackend* backend;

    /**
     * In-flight DBus work. Closing during a save needs a confirmation: the
     * setters go out one at a time, so a prefix of them is already persisted.
     */
    bool loading = false;
    bool saving = false;
    int readsInFlight = 0;

    /**
     * Set while the close confirmation is up. Its nested event loop still
     * delivers the setter replies, so the save's own boxes and accept() would
     * land on top of the prompt.
     */
    bool closePrompt = false;

    /**
     * Set once the close is confirmed. A reply can still be delivered between
     * QDialog::reject() and the dialog being destroyed.
     */
    bool closing = false;

    /**
     * Outcome of a save that finished while the close prompt was up, replayed
     * if the prompt is declined so the result is not lost.
     */
    QString pendingSaveError;
    std::function<void()> pendingSaveDone;

    /**
     * The form as last loaded or saved, so Apply and OK only write, and only
     * confirm, when something changed. Empty after a failed save, whose earlier
     * setters may have landed.
     */
    std::optional<FormValues> savedValues;

    // Current values
    int currentFPS;
    int currentSubsample;
    QString currentRoomID;
    bool currentGamingMode;
    QString currentGamingIcon;
    QString currentSyncingIcon;
    QString currentIdleIcon;
};

#endif // SETTINGSDIALOG_H

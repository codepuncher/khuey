#include "settingsdialog.h"
#include <QApplication>
#include <QDebug>

int main(int argc, char* argv[]) {
    QApplication app(argc, argv);

    qDebug() << "Creating SettingsDialog...";
    SettingsDialog* dialog = new SettingsDialog();
    qDebug() << "Dialog created successfully!";

    dialog->show();
    qDebug() << "Dialog shown, entering event loop...";

    return app.exec();
}

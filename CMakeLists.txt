cmake_minimum_required(VERSION 3.16)

project(plasma-hue-widget VERSION 0.1.0)

set(QT_MIN_VERSION "5.15.0")
set(KF5_MIN_VERSION "5.90.0")

find_package(ECM ${KF5_MIN_VERSION} REQUIRED NO_MODULE)
set(CMAKE_MODULE_PATH ${ECM_MODULE_PATH})

include(KDEInstallDirs)
include(KDECMakeSettings)
include(KDECompilerSettings NO_POLICY_SCOPE)

# Install the plasmoid
plasma_install_package(plasmoid org.kde.plasma.hue)

message(STATUS "")
message(STATUS "Plasma Hue Widget Configuration:")
message(STATUS "  Version: ${PROJECT_VERSION}")
message(STATUS "  Install prefix: ${CMAKE_INSTALL_PREFIX}")
message(STATUS "")
message(STATUS "After installation, remember to:")
message(STATUS "  1. Build the Go backend: cd backend && go build -o hue-sync ./cmd/hue-sync")
message(STATUS "  2. Start the backend service: ./backend/hue-sync")
message(STATUS "  3. Restart Plasma: kquitapp5 plasmashell && plasmashell &")
message(STATUS "")

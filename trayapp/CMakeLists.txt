cmake_minimum_required(VERSION 3.16)
project(hue-tray)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)

find_package(Qt6 REQUIRED COMPONENTS Core Widgets DBus)
find_package(KF6StatusNotifierItem REQUIRED)
find_package(KF6Notifications REQUIRED)

add_executable(hue-tray 
    main.cpp
    settingsdialog.cpp
    settingsdialog.h
)

target_link_libraries(hue-tray
    Qt6::Core
    Qt6::Widgets
    Qt6::DBus
    KF6::StatusNotifierItem
    KF6::Notifications
)

install(TARGETS hue-tray DESTINATION bin)

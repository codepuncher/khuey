cmake_minimum_required(VERSION 3.16)
project(test_settings_dialog)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_AUTOMOC ON)

find_package(Qt6 REQUIRED COMPONENTS Core Widgets DBus)

add_executable(test_settings_dialog 
    test_settings_dialog.cpp
    settingsdialog.cpp
)

target_link_libraries(test_settings_dialog 
    Qt6::Core
    Qt6::Widgets
    Qt6::DBus
)

#!/usr/bin/env python3
"""Test script to verify the Settings Dialog DBus calls work properly"""

import dbus

def test_get_sync_settings():
    bus = dbus.SessionBus()
    proxy = bus.get_object('org.kde.plasma.hue', '/org/kde/plasma/hue')
    iface = dbus.Interface(proxy, 'org.kde.plasma.hue')
    
    print("Testing GetSyncSettings...")
    result = iface.GetSyncSettings()
    print(f"✓ GetSyncSettings returned: {dict(result)}")
    return dict(result)

def test_get_bridge_settings():
    bus = dbus.SessionBus()
    proxy = bus.get_object('org.kde.plasma.hue', '/org/kde/plasma/hue')
    iface = dbus.Interface(proxy, 'org.kde.plasma.hue')
    
    print("\nTesting GetBridgeSettings...")
    result = iface.GetBridgeSettings()
    print(f"✓ GetBridgeSettings returned: {dict(result)}")
    return dict(result)

def test_get_selected_room():
    bus = dbus.SessionBus()
    proxy = bus.get_object('org.kde.plasma.hue', '/org/kde/plasma/hue')
    iface = dbus.Interface(proxy, 'org.kde.plasma.hue')
    
    print("\nTesting GetSelectedRoom...")
    result = iface.GetSelectedRoom()
    print(f"✓ GetSelectedRoom returned: {result}")
    return result

if __name__ == '__main__':
    try:
        sync_settings = test_get_sync_settings()
        bridge_settings = test_get_bridge_settings()
        selected_room = test_get_selected_room()
        
        print("\n✅ All DBus calls succeeded!")
        print("\nSettings Dialog should be able to load these values:")
        print(f"  FPS: {sync_settings.get('fps')}")
        print(f"  Subsample Width: {sync_settings.get('subsampleWidth')}")
        print(f"  Monitor: {sync_settings.get('monitor')}")
        print(f"  Bridge IP: {bridge_settings.get('bridgeIP')}")
        print(f"  Connected: {bridge_settings.get('connected')}")
        print(f"  Selected Room: {selected_room}")
        
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()

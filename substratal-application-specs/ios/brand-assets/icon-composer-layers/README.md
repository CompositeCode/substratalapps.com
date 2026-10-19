# Icon Composer layers

Import into Icon Composer (Xcode 26+) as one group per file, in order. Background is the icon tint `#E8E8F7`; keep the foreground layers flat colour — the system adds glass, specular and the dark/tinted/clear appearances. Export the resulting `AppIcon.icon` into the app target; the flat PNGs in `Assets.xcassets/AppIcon.appiconset` remain the fallback for older OS versions.

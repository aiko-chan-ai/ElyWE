## ElyWE - Animate Your Desktop Wallpaper

## Overview

ElyWE (Elysia free and open-source wallpaper engine) is a lightweight Go application designed to animate your desktop wallpaper on Windows. By leveraging [special code written by Gerald Degeneve](https://www.codeproject.com/articles/856020/draw-behind-desktop-icons-in-windows-plus), ElyWE can render a media player (MPV) behind the desktop.

## Current features:

- Allows setting videos as live wallpapers
- Starts with Windows
- Convenient right-click menu
- Extremely lightweight

## Usage

```
ElyWE [OPTIONS]

Options:
  -help                      Display help message
  -check                     Check if the MPV video player is installed correctly.
  -quit                      Kill all MPV processes.
  -set <filePath>            Set video file path.
  -install                   Install for right-click menu (Requires Admin).
  -uninstall                 Uninstall for right-click menu (Requires Admin).
  -enable_startup            Add shortcut to start with Windows.
  -disable_startup           Remove startup shortcut.
```

## Requirements

- Windows 8 or higher
- MPV video player installed

If MPV is not installed, you will encounter the following error:

```
Error: Your system does not have a valid video player (MPV) installed.
Please install it using the following command:
$ choco install mpv
If you have never used Chocolatey or installed a package with Chocolatey,
please see the following guide: https://dev.to/stephanlamoureux/getting-started-with-chocolatey-epo
```

## Installation

1. Download the latest release from the [releases page](https://github.com/aiko-chan-ai/ElyWE/releases) and save it in a secure location.
> [!IMPORTANT]
> The version in the release is the 64-bit version.
2. Open CMD in the directory where the file is saved.

## Basic Usage

To simply set a video as your wallpaper:

```
ElyWE --set "<video path>"
```

## Advanced Features

### Right-Click Menu

To add a right-click menu option for video files:

```
ElyWE --install
```

After installing, you can right-click any video file and select "Set as desktop background".

### Startup with Windows

To enable ElyWE to start with Windows:

```
ElyWE --enable_startup
```

### Uninstall Right-Click Menu

To remove the right-click menu option:

```
ElyWE --uninstall
```

### Disable Startup with Windows

To disable ElyWE from starting with Windows:

```
ElyWE --disable_startup
```

## Disclaimer

Please note: This software is intended for demo purposes, not for productive use. As such, it is not polished, well-written, configurable, or in any way convenient to use. Do whatever you want with it, at your own risk.

## Acknowledgements

- Special thanks to Gerald Degeneve for his [code](https://www.codeproject.com/articles/856020/draw-behind-desktop-icons-in-windows-plus).

---

Note: Used ChatGPT for some code snippets as well as this README.

---

Feel free to create an issue if you encounter any problems or have suggestions for improvements. Happy animating!
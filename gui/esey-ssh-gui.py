#!/usr/bin/env python3

import gi
import os
import sys

gi.require_version("Gtk", "3.0")
gi.require_version("Vte", "2.91")

from gi.repository import Gtk, Vte, GLib, Gdk


# --------------------------------------------------
# ROOT PATH DETECTION (SAFE)
# --------------------------------------------------

def get_root_dir():
    """
    Detect project root in all cases:
    - Running from source
    - Installed in /opt/esey-ssh-dev
    - Running from PyInstaller --onedir
    """
    if getattr(sys, "frozen", False):
        exe_dir = os.path.dirname(sys.executable)
        return os.path.abspath(os.path.join(exe_dir, ".."))
    current_dir = os.path.abspath(os.path.dirname(__file__))
    if os.path.basename(current_dir) == "gui":
        return os.path.abspath(os.path.join(current_dir, ".."))
    return current_dir


ROOT_DIR = get_root_dir()
BIN_DIR = os.path.join(ROOT_DIR, "bin")
LIB_DIR = os.path.join(ROOT_DIR, "gui", "lib")

SSHX_BIN = os.path.join(BIN_DIR, "sshx")
SSHX_KEY = os.path.join(BIN_DIR, "sshx-key")
SSHX_CPY = os.path.join(LIB_DIR, "sshx-cpy")
GIT_AUTH = os.path.join(LIB_DIR, "git-auth")
SSHX_RESET = os.path.join(LIB_DIR, "sshx-reset")


# --------------------------------------------------
# Terminal Tab
# --------------------------------------------------

class TerminalTab(Gtk.Box):
    def __init__(self, command=None):
        super().__init__(orientation=Gtk.Orientation.VERTICAL)

        self.terminal = Vte.Terminal()
        self.pack_start(self.terminal, True, True, 0)

        # Terminal options
        self.terminal.set_mouse_autohide(False)
        self.terminal.set_rewrap_on_resize(True)

        # Mouse right-click menu
        self.terminal.connect("button-press-event", self.on_right_click)

        # Keyboard shortcuts (Ctrl+Shift for copy/paste/select all)
        self.terminal.connect("key-press-event", self.on_key_press)

        # Spawn shell or command
        self.spawn(command)
        self.show_all()

    def spawn(self, command=None):
        if command:
            argv = command
        else:
            argv = [os.environ.get("SHELL", "/bin/bash")]

        self.terminal.spawn_async(
            Vte.PtyFlags.DEFAULT,
            os.environ.get("HOME"),
            argv,
            [],
            GLib.SpawnFlags.DEFAULT,
            None,
            None,
            -1,
            None,
            None,
        )

    # --------------------------------------------------
    # Right-click menu
    # --------------------------------------------------
    def on_right_click(self, widget, event):
        if event.type == Gdk.EventType.BUTTON_PRESS and event.button == 3:
            menu = Gtk.Menu()

            copy_item = Gtk.MenuItem(label="Copy")
            copy_item.connect("activate", lambda w: self.terminal.copy_clipboard())
            menu.append(copy_item)

            paste_item = Gtk.MenuItem(label="Paste")
            paste_item.connect("activate", lambda w: self.terminal.paste_clipboard())
            menu.append(paste_item)

            select_all_item = Gtk.MenuItem(label="Select All")
            select_all_item.connect("activate", lambda w: self.terminal.select_all())
            menu.append(select_all_item)

            menu.show_all()
            menu.popup_at_pointer(event)
            return True
        return False

    # --------------------------------------------------
    # Keyboard shortcuts
    # --------------------------------------------------
    def on_key_press(self, widget, event):
        ctrl = (event.state & Gdk.ModifierType.CONTROL_MASK)
        shift = (event.state & Gdk.ModifierType.SHIFT_MASK)
        key = Gdk.keyval_name(event.keyval).lower()

        # Ctrl+Shift shortcuts for terminal copy/paste/select all
        if ctrl and shift:
            if key == "c":
                self.terminal.copy_clipboard()
                return True
            elif key == "v":
                self.terminal.paste_clipboard()
                return True
            elif key == "a":
                self.terminal.select_all()
                return True
        return False


# --------------------------------------------------
# Main Window
# --------------------------------------------------

class SSHXGUI(Gtk.Window):
    def __init__(self):
        super().__init__(title="SSHX Ultimate Professional GUI")
        self.set_default_size(1600, 900)
        self.connect("destroy", Gtk.main_quit)

        main_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.add(main_box)

        toolbar = Gtk.Box(spacing=6)
        main_box.pack_start(toolbar, False, False, 0)

        self.notebook = Gtk.Notebook()
        main_box.pack_start(self.notebook, True, True, 0)

        # Core Buttons
        self.add_btn(toolbar, "Connect", self.connect_popup)
        self.add_btn(toolbar, "List", lambda b: self.run_cmd([SSHX_BIN, "--list"], "List"))
        self.add_btn(toolbar, "Doctor", lambda b: self.run_cmd([SSHX_BIN, "--doctor"], "Doctor"))
        self.add_btn(toolbar, "Version", lambda b: self.run_cmd([SSHX_BIN, "--version"], "Version"))
        self.add_btn(toolbar, "Help", lambda b: self.run_cmd([SSHX_BIN, "--help"], "Help"))

        # Advanced Buttons
        self.add_btn(toolbar, "Gen Key", self.gen_key_popup)
        self.add_btn(toolbar, "Copy Fingerprint", self.copy_fingerprint)
        self.add_btn(toolbar, "Git Auth", lambda b: self.run_cmd([GIT_AUTH], "GitAuth"))
        self.add_btn(toolbar, "SSHX Copy", self.sshx_copy_popup)
        self.add_btn(toolbar, "SSHX Reset", lambda b: self.run_cmd([SSHX_RESET], "Reset"))

        self.add_btn(toolbar, "Close Tab", self.close_tab)

        self.show_all()

    # --------------------------------------------------
    # Buttons
    # --------------------------------------------------
    def add_btn(self, box, label, callback):
        btn = Gtk.Button(label=label)
        btn.connect("clicked", callback)
        box.pack_start(btn, False, False, 0)

    def run_cmd(self, cmd, title):
        if not os.path.exists(cmd[0]):
            self.show_error(f"Command not found:\n{cmd[0]}")
            return
        self.new_tab(cmd, title)

    def new_tab(self, cmd=None, title="Terminal"):
        tab = TerminalTab(cmd)
        label = Gtk.Label(label=title)
        page = self.notebook.append_page(tab, label)
        self.notebook.set_current_page(page)
        self.show_all()

    # --------------------------------------------------
    # Popups
    # --------------------------------------------------
    def connect_popup(self, button):
        self.simple_input_popup(
            "Connect to SSHX",
            "Enter target (user@host):",
            lambda value: self.run_cmd([SSHX_BIN, value], value)
        )

    def gen_key_popup(self, button):
        self.simple_input_popup(
            "Generate SSH Key",
            "Enter Email:",
            lambda value: self.run_cmd([SSHX_KEY, value], "KeyGen")
        )

    def sshx_copy_popup(self, button):
        self.simple_input_popup(
            "SSHX Copy",
            "Enter user@host[:port]:",
            lambda value: self.run_cmd([SSHX_CPY, value], "SSHX Copy")
        )

    def simple_input_popup(self, title, label_text, callback):
        dialog = Gtk.Dialog(title=title, transient_for=self, flags=0)
        dialog.add_buttons("Cancel", Gtk.ResponseType.CANCEL,
                           "OK", Gtk.ResponseType.OK)

        box = dialog.get_content_area()

        label = Gtk.Label(label=label_text)
        entry = Gtk.Entry()

        box.pack_start(label, False, False, 5)
        box.pack_start(entry, False, False, 5)

        dialog.show_all()
        response = dialog.run()

        if response == Gtk.ResponseType.OK:
            value = entry.get_text().strip()
            if value:
                callback(value)

        dialog.destroy()

    # --------------------------------------------------
    # Fingerprint
    # --------------------------------------------------
    def copy_fingerprint(self, button):
        pubkey = os.path.expanduser("~/.ssh/id_ed25519.pub")
        if not os.path.exists(pubkey):
            self.show_error("Public key not found.")
            return

        self.new_tab(["ssh-keygen", "-lf", pubkey], "Fingerprint")

    # --------------------------------------------------
    # Tabs
    # --------------------------------------------------
    def close_tab(self, button):
        page = self.notebook.get_current_page()
        if page != -1:
            self.notebook.remove_page(page)

    def show_error(self, message):
        dialog = Gtk.MessageDialog(
            transient_for=self,
            flags=0,
            message_type=Gtk.MessageType.ERROR,
            buttons=Gtk.ButtonsType.OK,
            text=message,
        )
        dialog.run()
        dialog.destroy()


# --------------------------------------------------
# Run
# --------------------------------------------------

if __name__ == "__main__":
    win = SSHXGUI()
    Gtk.main()

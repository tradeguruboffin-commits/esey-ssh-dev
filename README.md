# SSHX – Simple SSH Manager (CLI + GUI)

SSHX is a lightweight SSH management tool designed for both developers and non-programmers.

It provides:

- ⚡ Simple CLI-based SSH connection management
- 🖥️ One-click GUI interface
- 🔐 Automatic SSH key generation
- 🔑 Automatic key copy for passwordless login
- 📁 Local host caching
- 🧹 Known_hosts cleanup support
- 🛠 Doctor self-check system

---

## 🚀 Features

### CLI Engine
- Connect using: `user@ip:port`
- Automatic SSH key generation (ed25519)
- First-time password login → auto key copy
- JSON-based host cache
- Remove host entries
- Optional interactive menu (fzf)
- Self-diagnosis (`--doctor`)

### GUI
- Clean SSH Control Panel
- Multi-tab terminal
- One-click connect
- Git authentication helper
- SSH reset tool
- Clipboard support
- Fully portable structure

---

## 📥 Installation

To install SSHX, clone the repository and run the installer:

```bash
git clone https://github.com/tradeguruboffin-commits/esey-ssh-dev.git
cd esey-ssh-dev
./bin/sshx-dev install
```

This will:

Link binaries to /usr/local/bin

Create a desktop entry for the GUI

Make sshx and sshx-gui globally accessible

---

## 🗑 Uninstall

To remove SSHX:

```bash
./bin/sshx-dev uninstall
```

## 🖥 Usage (CLI)

Connect to a server:
```bash
sshx user@ip:port
```
Remove a saved host:
```bash
sshx user@ip:port --remove
```
List saved hosts:
```bash
sshx --list
```
Interactive menu (requires fzf):
```bash
sshx --menu
```
Doctor check:
```bash
sshx --doctor
```

## 🖥 Usage (GUI)

After installation, launch:
```bash
sshx-gui
```
Or open SSHX from your system applications menu.

Enter:
```bash
user@ip:port
```
Click Connect.

---

## 📦 Dependencies

Required:
```bash
OpenSSH client

jq


# Optional:

fzf (for interactive menu)

```

## 🔒 Security Notes

SSH keys are stored in: ~/.ssh/

Host cache stored at: ~/.ssh/sshx.json

Key permissions are automatically fixed to 600

Known hosts entries are safely cleaned when removing hosts


## 👤 Author
```bash
Sumit
```


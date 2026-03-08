# iFacialMocap Proxy

A UDP relay for [iFacialMocap](https://www.ifacialmocap.com/) that sits between the iOS app and any number of receiving clients. It forwards face-tracking packets from a single iFacialMocap source to multiple destinations on your local network.

Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea) for an interactive terminal UI.

## Features

- Relays iFacialMocap UDP packets to multiple connected clients
- Interactive TUI with live connection status and packet stats
- Remembers the last connected remote across sessions

## Missing features

- Does not support the v2 packet format from iFacialMocap

## Install

```bash
go install codesnppet.dev/ifmproxy@latest
```

Or build from source:

```bash
./build.sh
```

## Usage

```bash
# Start on the default iFacialMocap port (49983)
./ifm-relay

# Start on a custom port
./ifm-relay -port 9000
```

Once running, type an IP address (or `connect <ip>`) to connect to an iFacialMocap device. Type `?` for available commands.

## License

MIT

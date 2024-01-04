# fakessh

A fake SSH tarpit that logs commands from attackers.

## Building

```shell
make
```

See `Makefile` for details.

## Running

No configuration file is required. See `fakessh -h` for available command-line options.

### Running as a systemd service

Copy `etc/fakessh.service` to your `/etc/systemd/system`, then run

```shell
systemctl daemon-reload
systemctl enable --now fakessh.service
```

Optionally (but recommended), copy `etc/logrotate.conf` to `/etc/logrotate.d/fakessh` to enable automatic log rotation.

## Example log

```text
2024/01/02 18:13:35 [conn] ip=157.245.113.75:48220
2024/01/02 18:13:36 [auth] ip=157.245.113.75:48220 version="SSH-2.0-Go" user="lichao" password="123456"
2024/01/02 18:13:36 [exec] ip=157.245.113.75:48220 cmd="uname -s -v -n -r -m"
2024/01/02 18:13:37 [exec] ip=157.245.113.75:48220 cmd="uptime -p"
2024/01/02 18:13:37 [exec] ip=157.245.113.75:48220 cmd="lspci | grep VGA | cut -f5- -d ' '"
2024/01/02 18:13:37 [exec] ip=157.245.113.75:48220 cmd="lspci | grep VGA -c"
2024/01/02 18:13:38 [exec] ip=157.245.113.75:48220 cmd="nvidia-smi -q | grep \"Product Name\" | head -n 1 | awk '{print $4, $5, $6, $7, $8, $9, $10, $11}'"
2024/01/02 18:13:38 [exec] ip=157.245.113.75:48220 cmd="lspci | grep \"3D controller\" | cut -f5- -d ' '"
2024/01/02 18:13:39 [exec] ip=157.245.113.75:48220 cmd="nvidia-smi -q | grep \"Product Name\" | awk '{print $4, $5, $6, $7, $8, $9, $10, $11}' | grep . -c "
2024/01/02 18:13:39 [exec] ip=157.245.113.75:48220 cmd="ip r | grep -Eo '[0-9]{1,3}.[0-9]{1,3}.[0-9]{1,3}.[0-9]{1,3}/[0-9]{1,2}' "
```

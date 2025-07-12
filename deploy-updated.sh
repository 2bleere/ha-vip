sudo systemctl stop ha-vip
sudo cp ~/ha-vip-linux-arm64-garp /usr/local/bin/ha-vip
sudo chmod +x /usr/local/bin/ha-vip
sudo systemctl start ha-vip
sudo journalctl -u ha-vip -f

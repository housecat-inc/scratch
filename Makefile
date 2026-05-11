BIN      := $(HOME)/go/bin/claude-control
PKG      := ./cmd/claude-control
PORT     ?= 8888
SERVICE  := claude-control.service
UNIT_DIR := $(HOME)/.config/systemd/user
UNIT     := $(UNIT_DIR)/$(SERVICE)

.PHONY: build install service deploy restart status logs uninstall

build:
	go build -o $(BIN) $(PKG)

install: build

$(UNIT_DIR):
	mkdir -p $(UNIT_DIR)

service: $(UNIT_DIR)
	@printf '%s\n' \
	  '[Unit]' \
	  'Description=claude-control web UI' \
	  'After=default.target' \
	  '' \
	  '[Service]' \
	  'ExecStart=$(BIN) --port $(PORT)' \
	  'Restart=on-failure' \
	  'RestartSec=2s' \
	  '' \
	  '[Install]' \
	  'WantedBy=default.target' \
	  > $(UNIT)
	systemctl --user daemon-reload
	systemctl --user enable $(SERVICE)

deploy: build service restart status

restart:
	systemctl --user restart $(SERVICE)

status:
	systemctl --user --no-pager status $(SERVICE)

logs:
	journalctl --user -u $(SERVICE) -f

uninstall:
	-systemctl --user disable --now $(SERVICE)
	rm -f $(UNIT)
	systemctl --user daemon-reload

go   := "asdf exec go"
lint := "asdf exec golangci-lint"
binary := "x11droid"
image  := "x11droid:latest"
vpkg   := "github.com/thereisnotime/x11droid/internal/version"

# show this help
default:
    @echo "\033[1;36m x11droid \033[0m— Waydroid in Podman on X11\n"
    @echo "\033[1;33m BUILD\033[0m"
    @echo "  \033[32mbuild\033[0m          compile the binary"
    @echo "  \033[32mrun\033[0m            build and launch the TUI"
    @echo "  \033[32minstall\033[0m        install binary to /usr/local/bin (sudo)"
    @echo "  \033[32mclean\033[0m          remove built binary"
    @echo "  \033[32mtidy\033[0m           go mod tidy"
    @echo ""
    @echo "\033[1;33m QUALITY\033[0m"
    @echo "  \033[32mtest\033[0m           go test ./..."
    @echo "  \033[32mtest-v\033[0m         go test -v ./..."
    @echo "  \033[32mlint\033[0m           golangci-lint run"
    @echo "  \033[32mvet\033[0m            go vet ./..."
    @echo "  \033[32mcheck\033[0m          vet + test + lint"
    @echo ""
    @echo "\033[1;33m IMAGE\033[0m"
    @echo "  \033[32mimage-build\033[0m    podman build -t {{image}}"
    @echo "  \033[32mimage-clean\033[0m    remove the container image"
    @echo ""
    @echo "\033[2m kernel modules are managed in-app: run x11droid, press s (Setup) → Load Modules\033[0m"

build:
    @{{go}} build -ldflags "\
      -X {{vpkg}}.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
      -X {{vpkg}}.Commit=$(git rev-parse --short HEAD 2>/dev/null || echo none) \
      -X {{vpkg}}.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      -o {{binary}} ./cmd/x11droid
    @echo "built {{binary}} — $(./{{binary}} --version)"

run: build
    ./{{binary}}

install:
    sudo install -m 0755 {{binary}} /usr/local/bin/{{binary}}
    @echo "installed to /usr/local/bin/{{binary}} (run with: sudo {{binary}})"

tidy:
    {{go}} mod tidy

vet:
    {{go}} vet ./...

test:
    {{go}} test ./...

test-v:
    {{go}} test -v ./...

lint:
    {{lint}} run ./...

check: vet test lint

clean:
    rm -f {{binary}}

image-build:
    podman build -t {{image}} .

image-clean:
    podman rmi -f {{image}}

# open an interactive Android (adb-style) root shell in an instance
adb name:
    sudo podman exec -it {{name}} bash -lc 'waydroid shell'

# capture ~25s of android logcat from a running instance to /tmp/lc.txt (debug)
logcat name:
    -sudo podman exec {{name}} bash -lc 'export DBUS_SESSION_BUS_ADDRESS=unix:path=/run/dbus/session_bus_socket; timeout 25 waydroid logcat' >/tmp/lc.txt 2>&1
    @echo "wrote /tmp/lc.txt ($(wc -l </tmp/lc.txt) lines)"

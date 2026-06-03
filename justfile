go   := "asdf exec go"
lint := "asdf exec golangci-lint"
binary := "x11droid"
image  := "x11droid:latest"

# show this help
default:
    @echo "\033[1;36m x11droid \033[0m— Waydroid in Podman on X11\n"
    @echo "\033[1;33m BUILD\033[0m"
    @echo "  \033[32mbuild\033[0m          compile the binary"
    @echo "  \033[32mrun\033[0m            build and launch the TUI"
    @echo "  \033[32minstall\033[0m        build and copy to ~/.local/bin"
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
    @echo "\033[1;33m KERNEL\033[0m"
    @echo "  \033[32mmodules-load\033[0m   load binder_linux (+ ashmem_linux)"
    @echo "  \033[32mmodules-unload\033[0m unload kernel modules"
    @echo "  \033[32mmodules-status\033[0m show loaded binder/ashmem modules"
    @echo ""
    @echo "\033[1;33m IMAGE\033[0m"
    @echo "  \033[32mimage-build\033[0m    podman build -t {{image}}"
    @echo "  \033[32mimage-clean\033[0m    remove the container image"
    @echo ""
    @echo "\033[1;33m WORKFLOW\033[0m"
    @echo "  \033[32msetup\033[0m          modules-load + image-build (first-time setup)"

build:
    {{go}} build -o {{binary}} ./cmd/x11droid

run: build
    ./{{binary}}

install: build
    cp {{binary}} /tmp/{{binary}}_install && mv /tmp/{{binary}}_install ~/.local/bin/{{binary}}
    @echo "installed to ~/.local/bin/{{binary}}"

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

modules-load:
    sudo modprobe binder_linux
    sudo modprobe ashmem_linux 2>/dev/null || true

modules-unload:
    sudo rmmod ashmem_linux 2>/dev/null || true
    sudo rmmod binder_linux 2>/dev/null || true

modules-status:
    @grep -E "binder|ashmem" /proc/modules || echo "no modules loaded"

setup: modules-load image-build
    @echo "\033[1;32m✓ ready — run 'just run'\033[0m"

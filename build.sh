#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

eval $(go env)
version=$(git describe --always --dirty)

pkg() {
	name="psmcli-$version-$GOOS-$GOARCH"

	rm -rf build
	dst="build/$name"

	mkdir -p "$dst"
	cp README.md LICENSE "$dst"
	go build -o "$dst/psmcli" -ldflags "-w -X main.Version $version"

	if [[ "$GOOS" == "windows" ]] ; then
		pushd build
		zip -r "../$name.zip" "$name"
		popd
	else
		tar zcvf "$name.tar.gz" -C build "$name"
	fi

	rm -rf build
}

case "${1:-default}" in
	pkg)
		pkg
		;;

	allpkg)
		rm -f *.tar.gz *.zip
		GOOS=linux GOARCH=amd64 pkg
		GOOS=linux GOARCH=386 pkg
		GOOS=windows GOARCH=amd64 pkg
		GOOS=windows GOARCH=386 pkg
		GOOS=darwin GOARCH=amd64 pkg
		;;

	default)
		GOBIN="$(pwd)/bin" go install -ldflags "-w -X main.Version $version"
		;;
esac

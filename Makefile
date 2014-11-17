PKG  = github.com/DevMine/srcanlzr
EXEC = srcanlzr

all: check test build

install:
	go install ${PKG}

build:
	go build -o ${EXEC} ${PKG}

test:
	go test ${PKG}/...

check:
	go vet ${PKG}/...
	golint ${GOPATH}/src/${PKG}

cover:
	go test -cover ${PKG}/...

clean:
	rm -f ./${EXEC}

###
# File: Makefile
# Author: Ming Cheng<mingcheng@outlook.com>
#
# Created Date: Tuesday, October 29th 2019, 11:05:07 am
# Last Modified: Thursday, January 16th 2020, 9:20:18 pm
#
# http://www.opensource.org/licenses/MIT
###

EXEC=./apcupsd_guarder
GO=go1.13.6

all: clean build

download:
	go get golang.org/dl/${GO}
	${GO} download

build: download clean
	@${GO} build -o ${EXEC} ./cmd/...

clean:
	@rm -f ${EXEC}
	@${GO} clean ./...

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

all: clean build

build: clean
	@go build -o ${EXEC} ./cmd/...

# install: build register.*
# 	cp -f $(shell pwd)/register.* /etc/systemd/system/
# 	cp -f $(shell pwd)/consul-register /usr/local/bin/

# uninstall:
# 	rm -f /usr/local/bin/consul-register
# 	rm -f /etc/systemd/system/register.*

clean:
	@rm -f ${EXEC}
	@go clean ./...

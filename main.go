/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package udevd is a library for working with uevent messages from the netlink
// socket.
package main

import (
	"log"
	"os"

	"github.com/mdlayher/kobject"
	"github.com/pkg/errors"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
}

func watch() error {
	client, err := kobject.New()
	if err != nil {
		return err
	}
	for {
		event, err := client.Receive()
		if err != nil {
			log.Printf("failed to receive event: %v", err)
			continue
		}
		if err = handle(event); err != nil {
			log.Printf("%v", err)
		}
	}
}

// nolint: gocyclo
func handle(event *kobject.Event) (err error) {
	switch event.Subsystem {
	case "block":
	case "bdi":
	default:
		return nil
	}

	var devname string
	var ok bool

	if devname, ok = event.Values["DEVNAME"]; !ok {
		return errors.Errorf("DEVNAME not found\n")
	}

	switch event.Action {
	case kobject.Add:
		log.Printf("%s: %s", "kobject add", devname)
		if err := SendRaw(event); err != nil {
			log.Printf("error sending: %+v", err)
		}
	case kobject.Remove:
		log.Printf("%s: %s", "kobject remove", devname)
		if err := SendRaw(event); err != nil {
			log.Printf("error sending: %+v", err)
		}
	default:
		log.Printf("unhandled action %q on %s", event.Action, devname)
	}

	return nil
}

func main() {
	if err := watch(); err != nil {
		log.Printf("failed watch uevents: %+v\n", err)
		os.Exit(1)
	}
}

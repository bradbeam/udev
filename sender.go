package main

import (
	"bytes"
	"log"
	"os"
	"unsafe"

	"github.com/mdlayher/kobject"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

/*
type Daemon struct {
	kClient  *kobject.Client
	nlClient int
}

func (d *Daemon) Send(event *kobject.Event) (err error) {
*/
func Send(event *kobject.Event) (err error) {
	/*
		var fd int
		if fd, err = unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_KOBJECT_UEVENT); err != nil {
			return nil, err
		}

		err = unix.Bind(fd, &unix.SockaddrNetlink{
			Family: unix.AF_NETLINK,
			Groups: unix.NETLINK_ADD_MEMBERSHIP,
			Pid:    uint32(os.Getpid()),
		})
	*/

	c, err := netlink.Dial(unix.NETLINK_USERSOCK, &netlink.Config{
		Groups: unix.NETLINK_ADD_MEMBERSHIP,
	})

	if err := c.JoinGroup(unix.NETLINK_ADD_MEMBERSHIP); err != nil {
		return err
	}

	if err != nil {
		log.Printf("failed to dial netlink: %v", err)
		return err
	}
	defer c.Close()

	req := netlink.Message{
		Data: event.Message,
		Header: netlink.Header{
			Sequence: uint32(event.Sequence),
		},
	}

	// Perform a request, receive replies, and validate the replies
	msgs, err := c.Send(req)
	if err != nil {
		log.Printf("failed to execute request: %v", err)
		return err
	}

	// Decode the copied request header, starting after 4 bytes
	// indicating "success"
	var res netlink.Message
	if err := (&res).UnmarshalBinary(msgs.Data[4:]); err != nil {
		log.Printf("failed to unmarshal response: %v", err)
		return err
	}

	log.Printf("res: %+v", res)
	return nil
}

func SendRaw(event *kobject.Event) (err error) {
	var fd int
	if fd, err = unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_USERSOCK); err != nil {
		return err
	}

	addr := &unix.SockaddrNetlink{
		Family: unix.AF_NETLINK,
		Groups: 0x2,
		Pid:    uint32(os.Getpid()),
	}

	if err = unix.Bind(fd, addr); err != nil {
		log.Printf("failed to bind to %+v", addr)
		return err
	}

	msg := bytes.Join(bytes.Split(event.Message, []byte{0x00})[1:], []byte{0x00})

	udevent := netlink.Message{
		Data: msg,
		Header: netlink.Header{
			Sequence: uint32(event.Sequence),
		},
	}
	var ml int
	ml = nlmsgLength(len(udevent.Data))
	fixMsg(&udevent, ml)

	libevent := netlink.Message{
		// libudev seems to send some 40byte message.
		// im not sure on the full message, but copied the similar bits
		// and changed everything else to null chars
		// Ex:
		// "libudev\0\376\355\312\376(\0\0\0(\0\0\0u\0\0\0*\30\204\241\0\0\0\0\0\0\0\0\0\0\0\0"
		Data: []byte("libudev\000\376\355\312\376(\000\000\000(\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000"),
		Header: netlink.Header{
			Sequence: uint32(event.Sequence),
		},
	}
	ml = nlmsgLength(len(libevent.Data))
	fixMsg(&libevent, ml)

	var fullmessage []byte
	var b []byte
	if b, err = libevent.MarshalBinary(); err != nil {
		return err
	}
	fullmessage = append(fullmessage, b...)
	if b, err = udevent.MarshalBinary(); err != nil {
		return err
	}
	fullmessage = append(fullmessage, b...)

	if err = unix.Sendmsg(fd, fullmessage, nil, addr, 0); err != nil {
		log.Printf("failed to send message %+v", event.Message)
		return err
	}

	return unix.Close(fd)
}

// Functions and values used to properly align netlink messages, headers,
// and attributes.  Definitions taken from Linux kernel source.

// #define NLMSG_ALIGNTO   4U
const nlmsgAlignTo = 4

// #define NLMSG_ALIGN(len) ( ((len)+NLMSG_ALIGNTO-1) & ~(NLMSG_ALIGNTO-1) )
func nlmsgAlign(len int) int {
	return ((len) + nlmsgAlignTo - 1) & ^(nlmsgAlignTo - 1)
}

// #define NLMSG_LENGTH(len) ((len) + NLMSG_HDRLEN)
func nlmsgLength(len int) int {
	return len + nlmsgHeaderLen
}

// #define NLMSG_HDRLEN     ((int) NLMSG_ALIGN(sizeof(struct nlmsghdr)))
var nlmsgHeaderLen = nlmsgAlign(int(unsafe.Sizeof(netlink.Header{})))

// #define NLA_ALIGNTO             4
const nlaAlignTo = 4

// #define NLA_ALIGN(len)          (((len) + NLA_ALIGNTO - 1) & ~(NLA_ALIGNTO - 1))
func nlaAlign(len int) int {
	return ((len) + nlaAlignTo - 1) & ^(nlaAlignTo - 1)
}

// Because this package's Attribute type contains a byte slice, unsafe.Sizeof
// can't be used to determine the correct length.
const sizeofAttribute = 4

// #define NLA_HDRLEN              ((int) NLA_ALIGN(sizeof(struct nlattr)))
var nlaHeaderLen = nlaAlign(sizeofAttribute)

// fixMsg updates the fields of m using the logic specified in Send.
func fixMsg(m *netlink.Message, ml int) {
	if m.Header.Length == 0 {
		m.Header.Length = uint32(nlmsgAlign(ml))
	}
}

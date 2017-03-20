package hid

/*
#cgo CFLAGS: -I/usr/local/include/hidapi
#cgo LDFLAGS: -L/usr/local/lib -lhidapi-libusb -lusb-1.0 -ludev
#include <stdio.h>
#include <stdlib.h>
#include "hidapi.h"

static void wcharcpy(wchar_t * dest, const wchar_t * source, size_t maxLen) {
	size_t i;
	for (i = 0; i < maxLen && source[i] != 0; i++) {
		dest[i] = source[i];	
	}
	dest[i] = 0;
}

//Return the character at the given index (because I don't known how to do it in cgo
static wchar_t wchar_at(const wchar_t * s, unsigned index) {
	return s[index];
}

*/
import "C"
import (
	"unsafe"
	"errors"
	"fmt"
)

const wchar_buffer_size = 256

var gInitState int

type Connection struct {
	cptr unsafe.Pointer
}

type DeviceInfo struct {
	/** Platform-specific device path */
	Path string
	/** Device Vendor ID */
	VendorID int
	/** Device Product ID */
	ProductID int
	/** Serial Number */
	SerialNumber string
	/** Device Release Number in binary-coded decimal,
		also known as Device Version Number */
	ReleaseNumber int
	/** Manufacturer String */
	ManufacturerStr string
	/** Product string */
	ProductStr string
	/** Usage Page for this Device/Interface
		(Windows/Mac only). */
	UsagePage int
	/** Usage for this Device/Interface
		(Windows/Mac only).*/
	Usage int
	/** The USB interface which this logical device
		represents. Valid on both Linux implementations
		in all cases, and valid on the Windows implementation
		only if the device contains more than one interface. */
	InterfaceNumber int
}

/**Print fields for debugging*/
func (dev * DeviceInfo) Print() {
	fmt.Printf("Path: %s\n", dev.Path)
	fmt.Printf("  Vendor : 0x%04x \"%s\"\n", dev.VendorID, dev.ManufacturerStr)
	fmt.Printf("  Product: 0x%04x \"%s\"\n", dev.ProductID, dev.ProductStr)
	fmt.Printf("  Serial: \"%s\"\n", dev.SerialNumber)
	fmt.Printf("  Release: %d\n", dev.ReleaseNumber)
	fmt.Printf("  UsagePage: %d\n", dev.UsagePage)
	fmt.Printf("  Usage: %d\n", dev.Usage)
	fmt.Printf("  InterfaceNumber: %d\n", dev.InterfaceNumber)		
}


//
// This must be called from a thread-safe context
//
func Init() error {
	if C.hid_init() != 0 {
		return errors.New("Failed to initialize HID library")
	} else {
		gInitState = 1
		return nil
	}
}

func Exit() {
	gInitState = 2
	C.hid_exit()	
}

func panicIfNotInit() {
	if gInitState == 0 {
		panic(errors.New("hidapi.Init() was not called"))
	} else if gInitState == 2 {
		panic(errors.New("hidapi.Exit() was already called!"))
	}
	//else OK
}

//HID_API_EXPORT hid_device * HID_API_CALL hid_open(unsigned short vendor_id, unsigned short product_id, const wchar_t *serial_number);
func Open(vendorId, productId int) (*Connection, error) {
	panicIfNotInit()
	
	ptr := C.hid_open(C.ushort(vendorId), C.ushort(productId), nil)
	if ptr != nil {
		return &Connection{
			cptr: unsafe.Pointer(ptr),
		}, nil
	} else {
		//TODO: check errno?
		return nil, errors.New(fmt.Sprintf("Unable to open HID device (vendor 0x%04X, product 0x%04X)", vendorId, productId))
	}	
}

/**Connect to this device.  Only the Path string is needed for this.*/
func (dev *DeviceInfo) Open() (*Connection, error) {
	panicIfNotInit()
	
	cpath := C.CString(dev.Path)	
	ptr := C.hid_open_path(cpath)
	C.free(unsafe.Pointer(cpath))
	
	if ptr != nil {
		return &Connection{
			cptr: unsafe.Pointer(ptr),
		}, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Unable to open HID device (path %s)", dev.Path))
	}
}


func (self *Connection) Close() {
	if self.cptr != nil {
		C.hid_close(self.cptr)
		self.cptr = nil
	}
}

func (self *Connection) errofIfClosed() error {
	if self.cptr == nil {
		return errors.New("HID connection already closed")
	} else {
		return nil
	}
}

func (self *Connection) last_error() error {
	//const wchar_t* HID_API_CALL hid_error(hid_device *device);
	wcptr := C.hid_error(self.cptr)
	var str string
	if wcptr != nil {
		str = wchar2str(wcptr)
	} else {
		//fallback to generic message
		str = "HID communication problem"
	}
	
	return errors.New(str)	
}

func (self *Connection) panic_ptr() {
	if self.cptr == nil {
		panic(errors.New("Connection already closed!"))
	}
}

func wcharBuf2str(wchars [wchar_buffer_size]C.wchar_t) string {
	//find null term
	term := C.wchar_t(0)
	slen := 0
	for ; slen < wchar_buffer_size && wchars[slen] != term; slen++ {
		
	}
	
	//TODO: this only works for ASCII!  use a proper conversion function
	raw := make([]byte, slen)
	for i := 0; i < slen; i++ {
		raw[i] = byte(int(wchars[i]) & 0xFF)
	}
	
	return string(raw)
}

func wchar2str(wstr *C.wchar_t) string {
	/*
	var wchars [wchar_buffer_size]C.wchar_t
	C.wcharcpy(&wchars[0], wcptr, C.size_t(wchar_buffer_size - 1))
	*/
	if wstr == nil {
		return ""
	}
	
	slen := uint(C.wcslen(wstr))
	
	//TODO: this only works for ASCII!  use a proper conversion function
	raw := make([]byte, slen)
	var i uint
	for i = 0; i < slen; i++ {
		c := C.wchar_at(wstr, C.uint(i))
		raw[i] = byte(c & 0xFF)
	}
	
	return string(raw)
}

/*
This function returns a linked list of all the HID devices
attached to the system which match vendor_id and product_id.
If @p vendor_id is set to 0 then any vendor matches.
If @p product_id is set to 0 then any product matches.
*/
func Enumerate(vendorId, productId int) ([]*DeviceInfo, error) {
	panicIfNotInit()
	
	head := C.hid_enumerate(C.ushort(vendorId), C.ushort(productId))
	
	list := make([]*DeviceInfo, 0, 16)
	
	//Walk the linked list
	cdev := head
	for cdev != nil {
		device := &DeviceInfo{
			VendorID: int(cdev.vendor_id),
			ProductID: int(cdev.product_id),
			ManufacturerStr: wchar2str(cdev.manufacturer_string),
			ProductStr: wchar2str(cdev.product_string),
			SerialNumber: wchar2str(cdev.serial_number),
			ReleaseNumber: int(cdev.release_number),
			UsagePage: int(cdev.usage_page),
			Usage: int(cdev.usage),
			InterfaceNumber: int(cdev.interface_number),
		}
	
		//check for nil just in case
		if cdev.path != nil {
			device.Path = C.GoString(cdev.path)
		} else {
			device.Path = "?"
		}
		
		list = append(list, device)
		cdev = cdev.next
	}
	
	if head != nil {
		C.hid_free_enumeration(head)
	}
	
	return list, nil
}

func (self *Connection) GetManufacturerStr() (string, error) {
	var wchars [wchar_buffer_size]C.wchar_t
	
	self.panic_ptr()
	
	//int hid_get_manufacturer_string(hid_device *device, wchar_t *string, size_t maxlen);
	if C.hid_get_manufacturer_string(self.cptr, &wchars[0], C.size_t(wchar_buffer_size - 1)) == 0 {
		return wcharBuf2str(wchars), nil		
	} else {
		return "", self.last_error()
	}
}

func (self *Connection) GetProductStr() (string, error) {
	var wchars [wchar_buffer_size]C.wchar_t
	
	self.panic_ptr()
	
	//int hid_get_product_string(hid_device *device, wchar_t *string, size_t maxlen);
	if C.hid_get_product_string(self.cptr, &wchars[0], C.size_t(wchar_buffer_size - 1)) == 0 {
		return wcharBuf2str(wchars), nil	
	} else {
		return "", self.last_error()
	}
}

func (self *Connection) GetSerialNumber() (string, error) {
	var wchars [wchar_buffer_size]C.wchar_t
	
	self.panic_ptr()
	
	//int hid_get_serial_number_string(hid_device *device, wchar_t *string, size_t maxlen);
	if C.hid_get_serial_number_string(self.cptr, &wchars[0], C.size_t(wchar_buffer_size - 1)) == 0 {
		return wcharBuf2str(wchars), nil
	} else {
		return "", self.last_error()
	}
}

/*
func (self *Connection) GetFeatureReport(reportID byte) ([]byte, error) {
	data := make([]byte, 65)
	data[0] = reportID
	for i := 1; i < 65; i++ {
		data[i] = byte(i)
	}	
	
//  int  hid_get_feature_report(hid_device *device, unsigned char *data, size_t length);
	res := C.hid_get_feature_report(self.cptr, (*C.uchar)(unsafe.Pointer(&data[0])), C.size_t(65))
	if res > 0 {
		return data, nil
	} else {
		return nil, self.last_error()
	}
}
*/

/**

*/
func (self *Connection) Read_timeout(numBytes, timeoutMillis int) ([]byte, error) {
	err := self.errofIfClosed()
	if err != nil {
		return nil, err
	}

	//int HID_API_EXPORT HID_API_CALL hid_read_timeout(hid_device *dev, unsigned char *data, size_t length, int milliseconds);
	data := make([]byte, numBytes)
	res := C.hid_read_timeout(self.cptr, (*C.uchar)(unsafe.Pointer(&data[0])), C.size_t(numBytes), C.int(timeoutMillis))
	if res == 0 {
		//timeout
		return nil, nil
	} else if res > 0 {
		return data, nil		
	} else {
		return nil, self.last_error()		
	}
}

/**
Send "Report ID" followed by data.  If Report ID 0x00 it will be discarded and
only the data bytes will be sent.
*/
func (self *Connection) Write(report []byte) error {
	err := self.errofIfClosed()
	if err != nil {
		return err
	}

	//int  HID_API_EXPORT HID_API_CALL hid_write(hid_device *device, const unsigned char *data, size_t length);
	res := C.hid_write(self.cptr, (*C.uchar)(unsafe.Pointer(&report[0])), C.size_t(len(report)))
	if int(res) == len(report) {
		return nil
	} else {
		return self.last_error()
	}
}







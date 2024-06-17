package device

import (
	"encoding/binary"
	"fmt"
	"log/slog"

	libusb "github.com/gotmc/libusb/v2"
)

const (
	// XREAL Light SLAM Camera and IMU (should be the same as OV580)
	XREAL_LIGHT_SLAM_CAM_VID = uint16(0x05a9)
	XREAL_LIGHT_SLAM_CAM_PID = uint16(0x0680)

	//XREAL Light RGB Camera
	XREAL_LIGHT_RGB_CAM_VID = uint16(0x0817)
	XREAL_LIGHT_RGB_CAM_PID = uint16(0x0909)

	//XREAL Light Audio
	XREAL_LIGHT_AUDIO_VID = uint16(0x0bda)
	XREAL_LIGHT_AUDIO_PID = uint16(0x4b77)
)

// See https://github.com/badicsalex/ar-drivers-rs/blob/master/src/nreal_light.rs#L604
var enableStreamingPacket = [34]byte{
	0x01, 0x00, // bmHint
	0x01,                   // bFormatIndex
	0x01,                   // bFrameIndex
	0x15, 0x16, 0x05, 0x00, // bFrameInterval (333333)
	0x00, 0x00, // wKeyFrameRate
	0x00, 0x00, // wPFrameRate
	0x00, 0x00, // wCompQuality
	0x00, 0x00, // wCompWindowSize
	0x65, 0x00, // wDelay
	0x00, 0x65, 0x09, 0x00, // dwMaxVideoFrameSize (615680)
	0x00, 0x80, 0x00, 0x00, // dwMaxPayloadTransferSize
	0x80, 0xd1, 0xf0, 0x08, // dwClockFrequency
	0x08, // bmFramingInfo
	0xf0, // bPreferredVersion
	0xa9, // bMinVersion
	0x18, // bMaxVersion
}

type xrealLightSLAMCameraFrame struct {
	/// Left frame data (640x480 grayscale pixels)
	Left []byte
	/// Right frame data (640x480 grayscale pixels)
	Right []byte
	/// Exact IMU timestamp when this frame was recorded
	TimeSinceBoot uint64
}

type xrealLightRGBCameraFrame struct {
	R []byte
	G []byte
	B []byte
	/// Exact IMU timestamp when this frame was recorded
	TimeSinceBoot uint64
}

type xrealLightCamera struct {
	initialized bool

	ctx *libusb.Context

	rgbCamera *libusb.DeviceHandle

	slamCamera *libusb.DeviceHandle
}

func (l *xrealLightCamera) connectAndInitialize() error {
	ctx, err := libusb.NewContext()
	if err != nil {
		return err
	}
	l.ctx = ctx

	devices, err := ctx.DeviceList()
	if err != nil {
		return fmt.Errorf("failed to enumerate USB devices: %w", err)
	}

	if len(devices) == 0 {
		return fmt.Errorf("no USB devices found: %v", devices)
	}

	rgbCameraDevices := []*libusb.Device{}
	slamCameraDevices := []*libusb.Device{}
	for _, device := range devices {
		descriptor, err := device.DeviceDescriptor()
		if err != nil {
			slog.Warn(fmt.Sprintf("failed to get device descriptor for %v, skip: %v", device, err))
			continue
		}
		if (descriptor.VendorID == XREAL_LIGHT_RGB_CAM_VID) && (descriptor.ProductID == XREAL_LIGHT_RGB_CAM_PID) {
			rgbCameraDevices = append(rgbCameraDevices, device)
		} else if (descriptor.VendorID == XREAL_LIGHT_SLAM_CAM_VID) && (descriptor.ProductID == XREAL_LIGHT_SLAM_CAM_PID) {
			slamCameraDevices = append(slamCameraDevices, device)
		}
	}

	slog.Debug(fmt.Sprintf("found rgb %v, slam %v", rgbCameraDevices, slamCameraDevices))

	if len(rgbCameraDevices) == 0 {
		return fmt.Errorf("no XREAL Light glass RGB cameras found")
	}

	if len(slamCameraDevices) == 0 {
		return fmt.Errorf("no XREAL Light glass SLAM cameras found")
	}

	for _, device := range rgbCameraDevices {
		// if l.rgbCameraDevicePath == nil {
		if len(rgbCameraDevices) > 1 {
			slog.Warn(fmt.Sprintf("multiple XREAL Light glass RGB cameras found, assuming to use the first one: %v", device))
		}
		// 	// l.rgbCameraDevicePath = &devicePath
		// }

		// if *l.rgbCameraDevicePath != devicePath {
		// 	continue
		// }

		deviceHandle, err := device.Open()
		if err != nil {
			return fmt.Errorf("failed to open RGB camera: %w", err)
		}
		l.rgbCamera = deviceHandle
	}

	// if l.rgbCamera == nil {
	// 	return fmt.Errorf("unable to match existing devices to device path %s", *l.rgbCameraDevicePath)
	// }

	for _, device := range slamCameraDevices {
		// if l.slamCameraDevicePath == nil {
		if len(slamCameraDevices) > 1 {
			slog.Warn(fmt.Sprintf("multiple XREAL Light glass SLAM cameras found, assuming to use the first one: %v", device))
		}
		// 	// l.slamCameraDevicePath = &devicePath
		// }

		// if *l.slamCameraDevicePath != devicePath {
		// 	continue
		// }

		deviceHandle, err := device.Open()
		if err != nil {
			return fmt.Errorf("failed to open SLAM camera: %w", err)
		}
		l.slamCamera = deviceHandle
	}

	// if l.slamCamera == nil {
	// 	return fmt.Errorf("unable to match existing devices to device path %s", *l.slamCameraDevicePath)
	// }

	return l.initialize()
}

func (l *xrealLightCamera) initialize() error {
	_, err := l.slamCamera.ControlTransfer( // see libusb_control_transfer
		0x21,    // LIBUSB_REQUEST_TYPE_CLASS | LIBUSB_RECIPIENT_INTERFACE
		0x01,    // the request field for the setup packet, UVC_SET_CUR
		0x02<<8, // the value field for the setup packet, UVC_VS_COMMIT_CONTROL
		0x01,    // the index field for the setup packet
		enableStreamingPacket[:],
		len(enableStreamingPacket),
		1000, // timeout, milliseconds
	)
	if err != nil {
		return fmt.Errorf("failed to send control transfer message to SLAM cam: %w", err)
	}

	_, err = l.rgbCamera.ControlTransfer( // see libusb_control_transfer
		0x21,    // LIBUSB_REQUEST_TYPE_CLASS | LIBUSB_RECIPIENT_INTERFACE
		0x01,    // the request field for the setup packet, UVC_SET_CUR
		0x02<<8, // the value field for the setup packet, UVC_VS_COMMIT_CONTROL
		0x01,    // the index field for the setup packet
		enableStreamingPacket[:],
		len(enableStreamingPacket),
		1000, // timeout, milliseconds
	)
	if err != nil {
		return fmt.Errorf("failed to send control transfer message to RGB cam: %w", err)
	}

	l.initialized = true

	return nil
}

func (l *xrealLightCamera) getFrameFromSLAMCamera() (*xrealLightSLAMCameraFrame, error) {
	var data []byte
	for {
		bulkData, receivedCount, err := l.slamCamera.BulkTransferIn(0x81, 615908, 0 /* unlimited timeout */)
		if err != nil {
			return nil, fmt.Errorf("failed to receive data from SLAM camera: %w", err)
		}
		if receivedCount == 615908 && bulkData[0] != 0 {
			data = bulkData
			break
		}
	}

	// Remove headers occurring every 0x8000 bytes (max transfer size)
	readIndex := 0
	writeIndex := 0
	for readIndex < len(data) {
		headerSize := int(data[readIndex])
		readIndex += headerSize

		// Calculate length to copy and adjust indices
		length := 0x8000 - (readIndex % 0x8000)
		readEnd := readIndex + length
		if readEnd > len(data) {
			readEnd = len(data)
		}

		copy(data[writeIndex:], data[readIndex:readEnd])
		readIndex += length
		writeIndex += length
	}

	// Truncate bulkData to the actual length after header removal
	data = data[:writeIndex]

	// Process bulk data to extract left and right frames
	var left, right []byte
	for i := 0; i < 480; i++ {
		left = append(left, data[(i*2)*640:(i*2+1)*640]...)
		right = append(right, data[(i*2+1)*640:(i*2+2)*640]...)
	}

	// Calculate timestamp from bulk data (simulation)
	var timeSinceBoot uint64
	if len(data) >= 640*480*2+8 {
		timeSinceBoot = binary.LittleEndian.Uint64(data[640*480*2 : 640*480*2+8])
	}
	timeSinceBoot = (timeSinceBoot/1000 + 37600) / 1000 // milliseconds

	return &xrealLightSLAMCameraFrame{
		Left:          left,
		Right:         right,
		TimeSinceBoot: timeSinceBoot,
	}, nil
}

func (l *xrealLightCamera) disconnect() error {
	l.initialized = false

	var errRGB error
	if l.rgbCamera != nil {
		errRGB := l.rgbCamera.Close()
		if errRGB == nil {
			l.rgbCamera = nil
		}
	}

	var errSLAM error
	if l.slamCamera != nil {
		errSLAM = l.slamCamera.Close()
		if errSLAM == nil {
			l.slamCamera = nil
		}
	}

	if errRGB != nil || errSLAM != nil {
		return fmt.Errorf("RGB err: %w; SLAM err: %w", errRGB, errSLAM)
	}

	if l.ctx != nil {
		if err := l.ctx.Close(); err != nil {
			return fmt.Errorf("failed to close libusb context")
		}
	}

	return nil
}

func slamFrameToImage(folderpath string, frame *xrealLightSLAMCameraFrame) (string, error) {
	return "", fmt.Errorf("not implemented: %s, %v", folderpath, frame)
}

package device

import (
	"fmt"
	"image"
	"image/jpeg"
	"log/slog"
	"os"
	"path/filepath"

	libusb "github.com/gotmc/libusb/v2"
)

const (
	// XREAL Light SLAM Camera and IMU (should be the same as OV580)
	XREAL_LIGHT_SLAM_CAM_VID    = uint16(0x05a9)
	XREAL_LIGHT_SLAM_CAM_PID    = uint16(0x0680)
	XREAL_LIGHT_SLAM_CAM_IF_NUM = 1

	//XREAL Light RGB Camera
	XREAL_LIGHT_RGB_CAM_VID    = uint16(0x0817)
	XREAL_LIGHT_RGB_CAM_PID    = uint16(0x0909)
	XREAL_LIGHT_RGB_CAM_IF_NUM = 0

	//XREAL Light Audio
	XREAL_LIGHT_AUDIO_VID = uint16(0x0bda)
	XREAL_LIGHT_AUDIO_PID = uint16(0x4b77)
)

// See https://github.com/badicsalex/ar-drivers-rs/blob/master/src/nreal_light.rs#L604
var enableSLAMStreamingPacket = []byte{
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

var enableRGBStreamingPacket = []byte{
	0x01, 0x00, // bmHint
	0x01,                   // bFormatIndex
	0x01,                   // bFrameIndex
	0x15, 0x16, 0x05, 0x00, // bFrameInterval (333333)
	0x00, 0x00, // wKeyFrameRate
	0x00, 0x00, // wPFrameRate
	0x00, 0x00, // wCompQuality
	0x00, 0x00, // wCompWindowSize
	0x65, 0x00, // wDelay
	0x00, 0xa9, 0xe6, 0x00, // dwMaxVideoFrameSize (15116544)
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
}

func (frame *xrealLightSLAMCameraFrame) toImage() (image.Image, image.Image) {
	left := bytesToImage(frame.Left, 640, 480, true /* isGray */)
	right := bytesToImage(frame.Right, 640, 480, true /* isGray */)
	return left, right
}

func (frame *xrealLightSLAMCameraFrame) WriteToFolder(folderpath string, prefixStr string) ([]string, error) {
	var filepaths []string

	imageLeft, imageRight := frame.toImage()
	if imageLeft != nil {
		filename := fmt.Sprintf("%s_left.jpeg", prefixStr)
		fpath := filepath.Join(folderpath, filename)
		if err := imageToJpegFile(imageLeft, fpath); err == nil {
			filepaths = append(filepaths, fpath)
		} else {
			return nil, err
		}
	}

	if imageRight != nil {
		filename := fmt.Sprintf("%s_right.jpeg", prefixStr)
		fpath := filepath.Join(folderpath, filename)
		if err := imageToJpegFile(imageRight, fpath); err == nil {
			filepaths = append(filepaths, fpath)
		} else {
			return nil, err
		}
	}

	return filepaths, nil
}

// bytesToImage converts []byte to image.Image in greyscale
func bytesToImage(data []byte, width, height int, isGray bool) image.Image {
	if len(data) == 0 {
		return nil
	}

	var img image.Image

	if isGray {
		grayImg := image.NewGray(image.Rect(0, 0, width, height))
		copy(grayImg.Pix, data)
		img = grayImg
	} else {
		rgbImg := image.NewRGBA(image.Rect(0, 0, width, height))
		copy(rgbImg.Pix, data)
		img = rgbImg
	}

	return img
}

func imageToJpegFile(img image.Image, filepath string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filepath, err)
	}
	defer f.Close()

	err = jpeg.Encode(f, img, nil)
	if err != nil {
		return fmt.Errorf("failed to write image to file %s: %w", filepath, err)
	}
	return nil
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
		}
		if (descriptor.VendorID == XREAL_LIGHT_SLAM_CAM_VID) && (descriptor.ProductID == XREAL_LIGHT_SLAM_CAM_PID) {
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
	if err := l.slamCamera.SetAutoDetachKernelDriver(true); err != nil {
		return fmt.Errorf("failed to SetAutoDetachKernelDriver(true) to SLAM cam: %w", err)
	}

	if err := l.slamCamera.ClaimInterface(XREAL_LIGHT_SLAM_CAM_IF_NUM); err != nil {
		return fmt.Errorf("failed to ClaimInterface(%d) to SLAM cam: %w", XREAL_LIGHT_SLAM_CAM_IF_NUM, err)
	}

	_, err := l.slamCamera.ControlTransfer( // see libusb_control_transfer
		0x21,    // LIBUSB_REQUEST_TYPE_CLASS | LIBUSB_RECIPIENT_INTERFACE
		0x01,    // the request field for the setup packet, UVC_SET_CUR
		0x02<<8, // the value field for the setup packet, UVC_VS_COMMIT_CONTROL
		0x01,    // the index field for the setup packet
		enableSLAMStreamingPacket,
		len(enableSLAMStreamingPacket),
		1000, // timeout, milliseconds
	)

	if err != nil {
		return fmt.Errorf("failed to send control transfer message to RGB cam: %w", err)
	}

	if err := l.rgbCamera.SetAutoDetachKernelDriver(true); err != nil {
		return fmt.Errorf("failed to SetAutoDetachKernelDriver(true) to RGB cam: %w", err)
	}

	if err := l.rgbCamera.ClaimInterface(XREAL_LIGHT_RGB_CAM_IF_NUM); err != nil {
		return fmt.Errorf("failed to ClaimInterface(%d) to RGB cam: %w", XREAL_LIGHT_RGB_CAM_IF_NUM, err)
	}

	_, err = l.rgbCamera.ControlTransfer( // see libusb_control_transfer
		0x21,    // LIBUSB_REQUEST_TYPE_CLASS | LIBUSB_RECIPIENT_INTERFACE
		0x01,    // the request field for the setup packet, UVC_SET_CUR
		0x02<<8, // the value field for the setup packet, UVC_VS_COMMIT_CONTROL
		0x01,    // the index field for the setup packet
		enableRGBStreamingPacket,
		len(enableRGBStreamingPacket),
		1000, // timeout, milliseconds
	)
	if err != nil {
		return fmt.Errorf("failed to send control transfer message to RGB cam: %w", err)
	}

	l.initialized = true

	return nil
}

func (l *xrealLightCamera) getRawBytesFromSLAMCamera() ([]byte, error) {
	data := make([]byte, 615908*2)
	for {
		receivedCount, err := l.slamCamera.BulkTransfer(0x81, data, len(data), 0 /* unlimited timeout */)
		if err != nil {
			return nil, fmt.Errorf("failed to receive data from SLAM camera: %w", err)
		}
		if receivedCount == 615908 && data[0] != 0 {
			data = data[:receivedCount]
			break
		}
		slog.Warn(fmt.Sprintf("got data size %d, skip and try again", receivedCount))
	}
	return data, nil
}

func (l *xrealLightCamera) getRawBytesFromRGBCamera() ([]byte, error) {
	data := make([]byte, 15116544*2)
	for {
		receivedCount, err := l.rgbCamera.BulkTransfer(0x81, data, len(data), 0 /* unlimited timeout */)
		if err != nil {
			return nil, fmt.Errorf("failed to receive data from RGB camera: %w", err)
		}
		if receivedCount != 0 {
			slog.Info(fmt.Sprintf("received %d bytes of data", receivedCount))
			data = data[:receivedCount]
			break
		}
		slog.Warn("got empty data, try again")
	}
	return data, nil
}

func (l *xrealLightCamera) getFrameFromSLAMCamera() (*xrealLightSLAMCameraFrame, error) {
	data, err := l.getRawBytesFromSLAMCamera()
	if err != nil {
		return nil, err
	}
	return BuildSLAMCameraFrame(data)
}

func BuildSLAMCameraFrame(data []byte) (*xrealLightSLAMCameraFrame, error) {
	if len(data) != 615908 || data[0] == 0 {
		return nil, fmt.Errorf("cannot handle received data that's different from size 615908")
	}

	// Remove headers occurring every 0x8000 bytes (max transfer size)
	readIndex := 0
	var dataCleaned []byte

	for readIndex < len(data) {
		headerSize := int(data[readIndex])

		readIndex += headerSize

		// Calculate length to copy and adjust indices
		length := 0x8000 - (readIndex % 0x8000)
		readEnd := readIndex + length
		if readEnd > len(data) {
			readEnd = len(data)
		}

		if headerSize == 12 {
			dataCleaned = append(dataCleaned, data[readIndex:readEnd]...)
		}

		readIndex = readEnd
	}

	data = dataCleaned

	// Process bulk data to extract left and right frames
	var left, right []byte
	for i := 0; i < 480; i++ {
		left = append(left, data[(i*2)*640:(i*2+1)*640]...)
		right = append(right, data[(i*2+1)*640:(i*2+2)*640]...)
	}

	return &xrealLightSLAMCameraFrame{
		Left:  left,
		Right: right,
	}, nil
}

func (l *xrealLightCamera) disconnect() error {
	l.initialized = false

	var errRGB error
	if l.rgbCamera != nil {
		l.rgbCamera.SetInterfaceAltSetting(XREAL_LIGHT_RGB_CAM_IF_NUM, 0)
		l.rgbCamera.ReleaseInterface(XREAL_LIGHT_RGB_CAM_IF_NUM)
		l.rgbCamera.AttachKernelDriver(XREAL_LIGHT_RGB_CAM_IF_NUM)
		errRGB = l.rgbCamera.Close()
		if errRGB == nil {
			l.rgbCamera = nil
		}
	}

	var errSLAM error
	if l.slamCamera != nil {
		l.slamCamera.SetInterfaceAltSetting(XREAL_LIGHT_SLAM_CAM_IF_NUM, 0)
		l.slamCamera.ReleaseInterface(XREAL_LIGHT_SLAM_CAM_IF_NUM)
		l.slamCamera.AttachKernelDriver(XREAL_LIGHT_SLAM_CAM_IF_NUM)
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

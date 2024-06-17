package device

import (
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
	l.initialized = true

	return nil
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

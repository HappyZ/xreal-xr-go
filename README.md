# xreal-xr-go

XREAL Light XR stuff. This is for personal interests and research.

### MacOS
Mac has a poor support on usb devices and needs to unbind with MacOS usb driver to use libusb and hidapi better. Will drop the support until we add Mac native implementations for now.

### Linux
```
sudo apt install libudev-dev libusb-1.0-0-dev libhidapi-dev libuvc-dev
```

###

Much of these are learned from https://git.9pm.me/happyz/ar-drivers-rs and https://git.9pm.me/happyz/NrealLightComms.

There is a really detailed protocol reverse engineering blog post at https://voidcomputing.hu/blog/good-bad-ugly/.

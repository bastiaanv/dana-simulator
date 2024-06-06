# Dana-i / DanaRS-v3 simulator

### Getting started

> Note: This simulator isn't able to do the native iOS security pincode prompt, due to limitations of the BLE stack

Before starting:

1. Ensure you have a raspberry Pi, (rPi) with Bluetooth support
1. The user must have `sudo` privileges on the rPi
    * If prompted during any of the steps below, enter your sudo password
1. Install the go language on the rPi
    * version 1.22.3 works
    * version 1.18.1 does not work

### Option 1

This version worked for a user - there may be a preferred solution.

This user prefers to have clones on their Mac and then rsync to the rPi.

Choose your method to get this clone

```
git clone https://github.com/bastiaanv/dana-simulator.git
```

into this folder on the rPi:

```
mkdir ~/dana-simulator
cd ~/dana-simulator
```

One time only, build the simulator & run it via:

```
go build
./simulator
```

Or for development you can run it directly via:

```
go run .
```

Use a phone with an app that supports DanaKit and select the dana option for the pump. Follow the prompts until the app is ready to scan for a pump.

The phone app should find the simulator and allow you to connect to it.

Keep the app in the foreground (unless you have something on the phone with a heartbeat) to keep the app going.


Before running `test-aci-auth-server` make sure that `go` and `actool` are in your
`$PATH`.

```
$ ./test-aci-auth-server start basic

{
	"rktKind": "auth",
	"rktVersion": "v1",
	"domains": ["127.0.0.1:48608"],
	"type": "basic",
	"credentials":
	{
		"user": "bar",
		"password": "baz"
	}
}

Ready, waiting for connections at https://127.0.0.1:48608
```

(You can run `test-aci-auth-server start` with either `none`, `basic` or `oauth` parameter)

Copy the snippet to `/etc/rkt/auth.d/test.json` and run `rkt
--insecure-skip-verify run
https://127.0.0.1:48608/<WHATEVER>/prog.aci`. The `rkt` output ought
to be something like:
```
# rkt --insecure-skip-verify run https://127.0.0.1:48608/basic1/prog.aci
rkt: fetching image from https://127.0.0.1:48608/basic1/prog.aci



3
2
1
BANG!
Sending SIGTERM to remaining processes...
Sending SIGKILL to remaining processes...
Unmounting file systems.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/dev/pts.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/dev/shm.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/sys.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/proc.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/dev/console.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/dev/tty.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/dev/urandom.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/dev/random.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/dev/full.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/dev/zero.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs/dev/null.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs.
Unmounting /proc/sys/kernel/random/boot_id.
Unmounting /opt/stage2/sha512-82d0d76f85d04a73e17a377c304ffbd8/rootfs.
All filesystems unmounted.
Halting system.
```

While the additional output from `test-aci-auth-server`:
```
Trying to serve "/basic10/prog.aci"
  serving
    done.
```

The `test-aci-auth-server start` with `oauth` will print something like this:
```
$ ./test-aci-auth-server start oauth

{
	"rktKind": "auth",
	"rktVersion": "v1",
	"domains": ["127.0.0.1:48805"],
	"type": "oauth",
	"credentials":
	{
		"token": "sometoken"
	}
}

Ready, waiting for connections at https://127.0.0.1:48805
```

To stop the `test-aci-auth-server` run stop command:
```
$ ./test-aci-auth-server stop https://127.0.0.1:60268
Response status: 200 OK
```

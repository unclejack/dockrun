dockrun
=======

**WARNING: This is a proof of concept! It's not meant to be used in production.**

This proof of concept 'docker run' wrapper attempts to make docker containers behave like unix processes.

It makes it easy to run docker containers under a process monitoring tool. It accepts the same options as `docker run`

### Purpose

This proof of concept is meant to show what improvements could be useful to docker.
It's also meant to be an attempt at making docker containers easier to monitor with process monitoring tools (systemd, supervisor & god).

It tries to bring the following features to docker run:

1. return the exit code of the process when the container exists
2. print logging output (stderr and stdout) to stdout when the container has exited
3. automatically remove the container when all operations are done
4. handle signals for process termination

### How it works

dockrun does the following in order to UNIX-ize docker containers:

1. It runs `docker run` with `-d` (detached) so that it can retrieve the container ID.
2. It runs `docker wait` to wait for the container to exit & to get the exit code of the process running inside it.
3. It sets up a signal handler to make the container behave like a UNIX process.
This handler makes sure that `docker stop` or `docker kill` gets executed in order to shut down the container and the process running within it.
4. It runs `docker logs` to retrieve the logs written to stdout and stderr of the process running in the container.
5. It runs `docker rm` to remove the container before exit when `-rm` is provided. This lets us make dockrun stateless across runs.
6. It exits with the same exit code as the process which was running in the container.

### Requirements

You need to have Go 1.1 installed to build this. You also need to have docker installed for this tool to work.

```
git clone https://github.com/unclejack/dockrun.git
cd dockrun
go build
```

dockrun was tested with docker 0.4.6.

### Usage

You'll find a few examples below.

Print "test" and automatically delete the container:
```dockrun -rm ubuntu echo "test"```
Start and expose a service running on port 5000 in the container via port 42850 on the host:
```dockrun -p 42850:5000 myrepo/coolservice```
Run a process which exits with exit code 3 and leaves the container behind:
```
$ dockrun ubuntu bash -c "exit 3"
$ echo $?
3
```

Observations:
1. docker run options ```-a``` and ```-i``` aren't supported by dockrun.
2. `-rm` makes dockrun automatically remove the resulting container

dockrun
=======

**WARNING: This is a proof of concept! It's not meant to be used in production.**

This proof of concept `docker run` wrapper attempts to make docker containers behave like unix processes.

It makes it easy to run docker containers under a process monitoring tool. It accepts the same options as `docker run`.

The final goal of this project is to merge most of this functionality into docker itself. `dockrun` may be used for development. Pull requests, bug reports and questions are welcome.

### Purpose

This proof of concept is meant to show what improvements could be useful to docker.
It's also meant to be an attempt at making docker containers easier to monitor with process monitoring tools (systemd, supervisor & god).

It tries to bring the following features to docker run:

1. return the exit code of the process when the container exists
2. print logging output (stderr and stdout) during the execution of the container
3. automatically remove the container when all operations are done if -rm is provided
4. handle signals for process termination (SIGTERM/SIGINT)
5. automatically commit the container and give it a repository name (optionally, a tag) if -commit reponame or -commit reponame:tag is provided

### How it works

dockrun does the following in order to UNIX-ize docker containers:

1. It runs `docker run` with `-cidfile` so that it can retrieve the container ID.
2. It attaches the stdin, stdout and stderr providede by `docker run` to make the container behave like a real process.
3. It runs `docker wait` to wait for the container to exit & to get the exit code of the process running inside it.
4. It sets up a signal handler to make the container behave like a UNIX process.
This handler makes sure that `docker stop` gets executed in order to shut down the container and the process running within it.
5. It commits the container if `-commit reponame` or `-commit reponame:tag` is provided if the container has exited with exit code 0.
5. It runs `docker rm` to remove the container before it exits when `-rm` is provided. This lets us make dockrun stateless across runs.
6. It exits with the same exit code as the process which was running in the container.

### Requirements

You need to have Go 1.1 installed to build this. You also need to have docker installed for this tool to work.

```
git clone https://github.com/unclejack/dockrun.git
cd dockrun
go build
```

dockrun was tested with docker 0.5.x.

### Usage

You'll find a few examples below.

Print "test" and automatically delete the container:
```
dockrun -rm ubuntu echo "test"
```

Start and expose a service running on port 5000 in the container via port 42850 on the host:
```
dockrun -p 42850:5000 myrepo/coolservice
```

Run a process which exits with exit code 3 and leaves the container behind:
```
$ dockrun ubuntu bash -c "exit 3"
$ echo $?
3
```

Start a container, pipe the string "test" into it, print it to stdout, echo the string "bla", exit with exit code 0, commit the container as image test:bla and remove the container:
```
echo "test" | dockrun -commit test:bla -i -rm ubuntu bash -c "cat && echo bla && exit 0"
```

Observations:

1. docker run options from the ```-a``` family aren't supported by dockrun.
2. `-rm` makes dockrun automatically remove the resulting container.
3. `-commit` makes dockrun commit the container to the given reponame or reponame:tag.
4. `dockrun` makes use of the `-cidfile` `docker run` option to keep track of the container ID.

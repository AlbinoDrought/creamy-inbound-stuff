# Creamy Inbound Stuff

<a href="https://hub.docker.com/r/albinodrought/creamy-inbound-stuff">
<img alt="albinodrought/creamy-inbound-stuff Docker Pulls" src="https://img.shields.io/docker/pulls/albinodrought/creamy-inbound-stuff">
</a>
<a href="https://github.com/AlbinoDrought/creamy-inbound-stuff/blob/master/LICENSE"><img alt="AGPL-3.0 License" src="https://img.shields.io/github/license/AlbinoDrought/creamy-inbound-stuff"></a>

Generate shareable links for people to upload files.

## Features

- Share public or password-protected links for inbound uploads
- Automatically disable links after an amount of time
- Automatically disable links after an amount of uploads

## Usage

Right now there are no configuration options.

### With Docker

```sh
docker run --rm -it \
    -v $(pwd)/foo/bar:/data \
    albinodrought/creamy-inbound-stuff
```

### Without Docker

```sh
./creamy-inbound-stuff
```

## Building

### With Docker

```sh
make image
```

### Without Docker

```sh
make build
```

# Hister

**Web history on steroids**

Blazing fast lookup of previously visited websites.

![hister screencast](assets/demo.gif)

## Features

 - Advanced [query language](https://blevesearch.com/docs/Query-String-Query/)
 - Blacklist & priority URL rules
 - Search keyword aliases for faster content retrieval
 - Web extension to automatically index visited websites

## Setup & run

 - Clone the repository
 - Build with `go build`
 - Run `./hister help` to list the available commands
 - Execute `./hister listen` to start the web application
 - Install the extension: [chrome](https://chromewebstore.google.com/detail/hister/cciilamhchpmbdnniabclekddabkifhb)


## Configuration

Settings can be configured in `~/.config/hister/config.yml` config file - don't forget to restart webapp after updating.

Execute `./hister create-config config.yml` to generate a configuration file with the default configuration values.


## Bugs

Bugs or suggestions? Visit the [issue tracker](https://github.com/asciimoo/hister/issues).


## License

AGPLv3

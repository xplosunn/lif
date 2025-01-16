# lif

Work in progress tool to facilitate infrastructure deployment as well as running locally.

## Vision

* User defines a program in their programming language of choice
* Has commands to:
  * run everything locally, has a web UI with:
    * console logs of every service, separated by service
    * network logs
  * run everything locally except some service of choice (so they can run that one separately) and has helpers to run that service connected to everything else
  * deploy
  * run a web UI that shows components and arrows between them
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

TBD:
* How to keep secrets required for deployment

To do for PoC:
1. Generate JSON with internal representation
1. Generate Dockerfile and run it
1. Have a way to read secrets somehow and actually deploy it

current task: 
  1. Add random generator dependency to create a random username and password for postgres when generating the docker compose.
  2. Generate the docker compose file. 
  
  After POC:

1. Migrate to using a webserver to generate the JSON and to start the deployment of custom resources. The other one we have to think about but this is the general talk:

    The PROBLEM: we have to run their code after the json has been generated and our initial design didn't account for that

    a. One way would be to run their code multiples times with different goals. One to generate the json and further times to generate each of the cloud resources.

    b. Instead of the user program spitting out a json, it runs a webserver, one endpoint returns the json. Another endpoint runs the code for custom resources by id

    c. Register custom resources in a different global var and figure out how to run the custom resource locally. 


2. Migrate to docker directly instead of docker compose. Because:
 - more flexible to start things not on docker. 
 - if we provide a web console we will want to display the logs for the separate services running on separate tabs or something
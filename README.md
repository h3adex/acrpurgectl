# Azure ACR Purge Control 🗑️

``acrpurgectl`` is a tool that extends the az acr purge deletion command.
It parses all images from the Kubernetes (k8s) contexts that you provide and ensures that no
image currently running in your cluster is deleted.

## CLI Commands

Here are the available CLI commands and their descriptions:

| Flag           | Default Value | Description                                                                                            |
|----------------|---------------|--------------------------------------------------------------------------------------------------------|
| `registry`     | ""            | Name of the Azure Container Registry.                                                                  |
| `repository`   | ""            | Name of the repository in your registry.                                                               |
| `subscription` | ""            | ID of the subscription. If not specified it will use the default one.                                  |
| `contexts`     | ""            | Comma-separated list of Kubernetes contexts. Deletion process will not start if image is running here. |
| `all-contexts` | false         | Deletion process will not start if image is running in a cluster from your kubeconfig contexts.        |
| `ago`          | "360d"        | Time duration in the past. Number followed by a duration type: 's' for seconds, 'm' for minutes, etc.  |
| `dry-run`      | false         | Perform a dry run, print tags to be deleted but do not delete them.                                    |


## Usage with Docker

You can use Azure ACR Purge with Docker by running the Docker image and logging into your Azure account. Here's how you can do it:

Run the Docker image and access the interactive shell:

```sh
docker run -it --rm ghcr.io/h3adex/acrpurgectl:latest
```

Inside the container's interactive shell, log in to your Azure account using the Azure CLI:

```sh
az login
```

Execute Azure ACR Purge with your desired parameters. For example:

```sh
./acrpurgectl --repository test/repo-a --registry testregistry --subscription 1111-2222-3333-4444 --ago 360d
```

If you want to use the `--contexts` option, you need to share your local kubeconfig file with the Docker container, to allow Azure ACR Purge to access your Kubernetes contexts:

```sh
docker run -it --rm -v /path/to/your/.kube/config:/root/.kube/config ghcr.io/h3adex/acrpurgectl:latest
```

Then execute Azure ACR Purge with the `--contexts` parameter:

```sh
./acrpurgectl --repository test/repo-a --registry testregistry --subscription 1111-2222-3333-4444 --ago 360d --contexts context1,context2
```

This will initiate the process to delete old images from the specified repository in your Azure Container Registry based on the provided parameters. 
Make sure to tailor the commands to your specific needs and repositories. Happy cleaning! 🧹🐳

# Azure ACR Purge Control 🗑️

``acrpurgectl`` is a tool that extends the az-cli acr deletion command.
It parses all images from the Kubernetes (k8s) contexts that you provide and ensures that no
image currently running in your cluster is deleted.

## Key Features
- Takes into account the list of Kubernetes contexts during the deletion process, ensuring no running image is deleted.
- Eliminates the need to pay for ACR tasks.
- Offers a familiar workflow inspired by the "terraform plan" and "terraform apply" approach.
- Allows for the use of the "dry-run" command to preview tags before actual deletion, instilling confidence in your actions.

## CLI Commands

Here are the available CLI commands and their descriptions:

| Command                            | Description                                                                                                                                            |
|------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------|
| `--registry <registry_name>`       | Set the name of the Azure Container Registry.                                                                                                          |
| `--repository <repository_name>`   | Set the name of the repository in your Azure Container Registry.                                                                                       |
| `--subscription <subscription_id>` | Set the ID of the Azure subscription. If not specified, the default one will be used.                                                                  |
| `--timestamp <cutoff_timestamp>`   | Set the cutoff timestamp. All images before this timestamp will be deleted. Default: 01/01/2024.                                                       |
| `--delay <delay_in_seconds>`       | Set the delay (in seconds) between deletion requests. Default: 1 second.                                                                               |
| `--contexts <context_list>`        | Comma-separated list of Kubernetes contexts. The deletion process will not start if any 'imageToDelete' is running in a cluster from the context list. |
| `--dry-run`                        | Perform a dry run, printing the tags to be deleted but do not delete them.                                                                             |

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
./acrpurgectl --repository test/repo-a --registry testregistry --subscription 1111-2222-3333-4444 --timestamp 01/02/2021
```

If you want to use the `--contexts` option, you need to share your local kubeconfig file with the Docker container, to allow Azure ACR Purge to access your Kubernetes contexts:

```sh
docker run -it --rm -v /path/to/your/.kube/config:/root/.kube/config ghcr.io/h3adex/acrpurgectl:latest
```

Then execute Azure ACR Purge with the `--contexts` parameter:

```sh
./acrpurgectl --repository test/repo-a --registry testregistry --subscription 1111-2222-3333-4444 --timestamp 01/02/2021 --contexts context1,context2
```

This will initiate the process to delete old images from the specified repository in your Azure Container Registry based on the provided parameters. 
Make sure to tailor the commands to your specific needs and repositories. Happy cleaning! 🧹🐳

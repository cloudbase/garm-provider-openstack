# Garm External Provider For OpenStack

The OpenStack external provider allows [garm](https://github.com/cloudbase/garm) to create Linux and Windows runners on top of OpenStack virtual machines.

## Build

Clone the repo:

```bash
git clone https://github.com/cloudbase/garm-provider-openstack
```

Build the binary:

```bash
cd garm-provider-openstack
go build .
```

Copy the binary on the same system where ```garm``` is running, and [point to it in the config](https://github.com/cloudbase/garm/blob/main/doc/providers.md#the-external-provider).

## Configure

The config file for this external provider is a simple toml used to configure the credentials needed to connect to your OpenStack cloud and some additional information about your environment.

A sample config file can be found [in the testdata folder](./testdata/config.toml).

## Tweaking the provider

Garm supports sending opaque json encoded configs to the IaaS providers it hooks into. This allows the providers to implement some very provider specific functionality that doesn't necessarily translate well to other providers. Features that may exists on Azure, may not exist on AWS or OpenStack and vice versa.

To this end, this provider supports the following extra specs schema:

```json

{
    "$schema": "http://cloudbase.it/garm-provider-openstack/schemas/extra_specs#",
    "type": "object",
    "description": "Schema defining supported extra specs for the Garm OpenStack Provider",
    "properties": {
        "security_groups": {
            "type": "array",
            "items": {
                "type": "string"
            }
        },
        "network_id": {
            "type": "string",
            "description": "The tenant network to which runners will be connected to."
        },
        "storage_backend": {
            "type": "string",
            "description": "The cinder backend to use when creating volumes."
        },
        "boot_from_volume": {
            "type": "boolean",
            "description": "Whether to boot from volume or not. Use this option if the root disk size defined by the flavor is not enough."
        },
        "boot_disk_size": {
            "type": "integer",
            "description": "The size of the root disk in GB. Default is 50 GB."
        },
        "use_config_drive": {
            "type": "boolean",
            "description": "Use config drive."
        },
        "enable_boot_debug": {
            "type": "boolean",
            "description": "Enable cloud-init debug mode. Adds `set -x` into the cloud-init script."
        },
        "allow_image_owners": {
            "type": "array",
            "items": {
                "type": "string"
            },
            "description": "A list of image owners to allow when creating the instance. If not specified, all images will be allowed." 
        }
    },
	"additionalProperties": false
}
```

An example extra specs json would look like this:

```json
{
    "boot_from_volume": true,
    "security_groups": ["allow_ssh", "allow_web"],
    "network_id": "542b68dd-4b3d-459d-8531-34d5e779d4d6",
    "storage_backend": "cinder_nvme",
    "boot_disk_size": 150,
    "use_config_drive": false
}
```

To set it on an existing pool, simply run:

```bash
garm-cli pool update --extra-specs='{"network_id": "542b68dd-4b3d-459d-8531-34d5e779d4d6"}' <POOL_ID>
```

You can also set a spec when creating a new pool, using the same flag.

Workers in that pool will be created taking into account the specs you set on the pool.
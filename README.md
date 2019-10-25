terrafrom-migrate
===

Migrations for your Terraform state

**NOTE:** There are a few hard-coded things and custom logic for our case.
Working on it to make it easy for other people to adopt.


### How to install?

Install with:

```
go get -u carwow/terraform-migrate
```


### What it requires?

It currently only works within CircleCI. Requires a `CIRCLE_TOKEN` environment
variable and a `backend.tf` file where your terraform backend is defined.

Migrations go in the `../../migrations` directory. They are prefixed with a
number with no leading zeros e.g. `1_import_resource.sh`, and are an executable
file.


### How it works?

After running `terraform init` but before `terraform plan`, run:

```
terraform-migrate init && terraform-migrate plan
```

This sets up a lock on CircleCI's environment variables, disables the terraform
backend, by moving the `backend.tf` file to `backend.tf.disable`, and
re-initializes terraform with `-force-copy` which copies the terraform state
locally. From this point on all terraform commands that change the state apply
on the local state so you can do whatever you what on your migration file.

Then on your master branch, between the `terraform init` and `terraform plan`
commands, instead of running `terraform-migrate init && terraform-migrate
plan`, run:

```
terraform-migrate apply
```

This will run the migration directly on the remote state.

See an example CircleCI configuration file and an example migration file in the
examples directory.

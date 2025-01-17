---
title: "Prologue"
description: "An introduction into configuring Authelia."
lead: "An introduction into configuring Authelia."
date: 2022-03-20T12:52:27+11:00
draft: false
images: []
menu:
  configuration:
    parent: "prologue"
weight: 100100
toc: true
---

## Documentation

We document the configuration in two ways:

1. The [YAML] configuration template
   [config.template.yml](https://github.com/authelia/authelia/blob/master/config.template.yml) has comments with very
   limited documentation on the effective use of a particular option. All documentation lines start with `##`. Lines
   starting with a single `#` are [YAML] configuration options which are commented to disable them or as examples.
2. This documentation site. Generally each section of the configuration is in its own section of the documentation
   site. Each configuration option is listed in its relevant section as a heading, under that heading generally are two
   or three colored labels.
   * The `type` label is purple and indicates the [YAML] value type of the variable. It optionally includes some
     additional information in parentheses.
   * The `default` label is blue and indicates the default value if you don't define the option at all. This is not the
     same value as you will see in the examples in all instances, it is the value set when blank or undefined.
   * The `required` label changes color. When required it will be red, when not required it will be green, when the
     required state depends on another configuration value it is yellow.

## Validation

Authelia validates the configuration when it starts. This process checks multiple factors including configuration keys
that don't exist, configuration keys that have changed, the values of the keys are valid, and that a configuration
key isn't supplied at the same time as a secret for the same configuration option.

You may also optionally validate your configuration against this validation process manually by using the
`authelia validate-config` command. This command is useful prior to upgrading to prevent configuration changes from
impacting downtime in an upgrade. This process does not validate integrations, it only checks that your configuration
syntax is valid.

```bash
authelia validate-config --config configuration.yml
```

[YAML]: https://yaml.org/

# The Graphite Input

The InfluxDB Graphite input accepts Graphite metrics in plaintext protocol.

- [Configuration](#configuration)
    - [Basic Config](#basic-config)
    - [Batch Settings](#batch-settings)
    - [UDP/IP OS Buffer Sizes](#UDP/ip-os-buffer-sizes)
    - [Input Tags](#input-tags)
- [Templates](#templates)
    - [Default Behavior](#default-behavior)
- [Pattern Parsing](#pattern-parsing)
    - [Empty Tokens](#empty-tokens)
    - [Wildcard](#wildcard)
    - [Multiple Matching Tokens](#multiple-matching-tokens)
    - [Template Tags](#template-tags)
- [Multiple Templates](#multiple-templates)
    - [Filters](#filters)
    - [Base Template](#base-template)
- [Advanced Configuration Examples](#advanced-configuration-examples)

## Configuration

Each Graphite input allows the binding address, target database, and protocol to be set.
If the database does not exist, it will be created automatically when the input is initialized.

Multiple Graphite inputs can be configured to listen on different ports with different settings by specifying multiple `[[graphite]]` sections in the InfluxDB configuration file.

#### Basic config

Here is an example of a basic Graphite template configuration:

```
[[graphite]]
  enabled = true
  # bind-address = ":2003"
  # protocol = "tcp"
  # database = "graphite"

  # batch-size = 5000
  # batch-pending = 10
  # batch-timeout = "1s"
  # udp-read-buffer = 0

  ### If matching multiple measurement files, this string will be used to join the matched values.
  # separator = "."

  ### Default tags that will be added to all metrics.  These can be overridden at the template level
  ### or by tags extracted from metric
  # tags = ["region=us-east", "zone=1c"]

  ### Each template line requires a template pattern.  It can have an optional
  ### filter before the template and separated by spaces.  It can also have optional extra
  ### tags following the template.  Multiple tags should be separated by commas and no spaces
  ### similar to the line protocol format.  The can be only one default template.
  # templates = [
  #   "*.app env.service.resource.measurement",
  #   # Default template
  #   "server.*",
 #]
```

#### Batch settings

The Graphite input performs internal batching of the points each input receives.

The default `batch-size` is 1000, `batch-pending` factor is 5, with a `batch-timeout` of 1 second.
This means the input will write a batch every time a maximum of 1000 points are received within the timeout window of 1 second.
Increasing the batch size is the easiest way to improve performance.

If a batch has not reached 1000 points within 1 second of the first point being added to a batch, it will emit that batch regardless of size.
Points can be lost in the window from when they are received to when the batch timeout is triggered (assuming the number of points received in the window is less than the batch size).
Adjusting the batch timeout window to a longer window will increase the window where points may be lost at the benefit of reducing the number of write operations, which can be useful in low power/low throughput situations.

The pending batch factor limits the number of batches held in memory at once.
New points will not be accepted when the number of pending batches waiting to be written is equal to the pending batch factor.
Increasing the batch pending factor will increase the ability for the input to handle spikes in traffic at the expense of higher memory use.

#### UDP/IP OS Buffer sizes

Using the UDP protocol while running Linux or FreeBSD, may require adjustment of the UDP buffer size limit with the `udp-read-buffer` setting
For more detail, see [the note on UDP/IP OS buffer sizes in the UDP plugin readme](../udp/README.md#a-note-on-udpip-os-buffer-sizes).

#### Input Tags

Tags can be set for all points received by a Graphite input.
They should be specified as a TOML list of tag keys and values in InfluxDB line protocol format.

```
tags = ["region=us-east", "zone=1c"]
```

Note that tags defined by templates will overwrite any matching input tags.
This makes input tags useful for defining default values for all points received by a plugin.

## Templates

Templates are the main feature of InfluxDB's Graphite input.
They allow incoming points formatted in Graphite's plaintext metric format to be transformed into InfluxDB points.
Templates are composed of the following three pieces: `[filter] <pattern> [tags]`
- an optional [filter](#filter) to match which points to which the template will apply
- a required [pattern](#pattern) to translate a Graphite metric path into InfluxDB format
- an optional set of [tags](#tags) to add to all Graphite metrics matching the filter

Here is an example of the configuration to define a single template.

```
templates = ["*.app env..service.measurement.field app=load_balancer"]
```

Each Graphite input can define [multiple templates](#multiple-templates), so a single Graphite input can parse a diverse stream of metrics using multiple filters and patterns.

#### Default Behavior

By default, if no templates are specified or a metric does not match any template filters, the Graphite metric path is used as the InfluxDB measurement name and the field key will be `value`.
No tags will be assigned.

For example, sending this Graphite metric with no template defined:

```
servers.localhost.cpu.loadavg.10 12 1459382400
-------------------------------- -- ----------
                |                |       |
          metric path          field timestamp
                               value
```

Will be translated into this point (in [InfluxDB line protocol](https://docs.influxdata.com/influxdb/latest/write_protocols/line/) format):

```
servers.localhost.cpu.loadavg.10 value=12 1459382400
```

Notice the InfluxDB measurement name is the same as the full Graphite metric path, `servers.localhost.cpu.loadavg.10`, with no extracted tags.

While this default setup works, it is not the ideal way to store measurements in InfluxDB since it does not take advantage of tags.
This means queries will always need to use regular expressions to select multiple measurements at once, which does not scale well.

To extract tags from metrics, one or more templates with a pattern must be configured to parse the Graphite metric path.

## Pattern Parsing

A pattern is the only required piece of a template.
Patterns can extract the following information from each Graphite metric:
- a measurement name
- tag values (for a set of tag keys)
- a field name

The format of a pattern is similar to the format of Graphite metric path.
Periods (`.`) are used to reference parts of a Graphite metric path and assign those parts to tokens.
The tokens can be either `measurement`, `field`, or a string representing a tag key.
Each part of the Graphite metric path is matched to the token at the same position on the pattern.
The matching part of the Graphite metric then becomes the value of the corresponding token.

If no `measurement` is specified in a pattern, the full Graphite metric path is used as the measurement name.

If no `field` is specified in a pattern, the field name will always default to `value`.

For example, sending this Graphite metric to InfluxDB's Graphite input:

```
localhost.cpu.loadavg.10 12 1459382400
```

And applying the following template, which only has a pattern:

```
host.measurement.field.cpu
```

Will translated the metric into the following point (in [InfluxDB line protocol](https://docs.influxdata.com/influxdb/latest/write_protocols/line/) format):

```
cpu,host=localhost,cpu=10 loadavg=12 1459382400
```

#### Empty Tokens

Empty tokens in a pattern cause the matching part of the Graphite metric to be skipped.

```
# Graphite metric
servers.localhost.cpu.load.loadavg.10 12 1459382400
# Template
.host.measurement..field.cpu
# Resulting point in InfluxDB line protocol
cpu,host=localhost,cpu=10 loadavg=12 1459382400
```

#### Wildcard

A trailing asterisk on the last measurement or field token appends any remaining parts of the Graphite metric path to that token.
This is known as a wildcard token.
Only one wildcard may be used per pattern and it must be the last token.

```
# Graphite metric
servers.localhost.cpu.load.loadavg.10 12 1459382400
# Template
.host.measurement.field*
# Resulting point in InfluxDB line protocol
cpu,host=localhost load.loadavg.10=12 1459382400
```

```
# Graphite metric
servers.localhost.cpu.load.loadavg.10 12 1459382400
# Template
.host.measurement*
# Resulting point in InfluxDB line protocol
cpu.load.loadavg.10,host=localhost value=12 1459382400
```

Note the field name is `value` in the example above because the `field` token is not specified.

#### Multiple Matching Tokens

Tokens can be specified multiple times in a pattern to provide more control over naming.
Every identical token will cause the matching parts of the Graphite metric path to be joined together with a separator separating each part.

The `separator` is defined in the configuration file.
By default, value of the separator is a period (`.`).
The underscore (`_`) is frequently used as an alternative separator, since periods have another meaning in InfluxQL (separating identifiers in a `FROM` statement) which can lead to confusing query syntax.

```
# Graphite metric
servers.localhost.localdomain.cpu.cpu0.user 402 1459382400
# Template with default "." separator
.host.host.measurement.cpu.measurement
# Resulting point in InfluxDB line protocol
cpu.user,host=localhost.localdomain,cpu=cpu0 value=402 1459382400
```

```
# Graphite metric
servers.localhost.localdomain.cpu.cpu0.user 402 1459382400
# Template with "_" separator
field.host.host.measurement.cpu.field
# Resulting point in InfluxDB line protocol
cpu,host=localhost_localdomain,cpu=cpu0 servers_user=402 1459382400
```

#### Template Tags

Additional tags can be defined per template.
They must come after the pattern and separated with a space.
The tags must have the same format as InfluxDB line protocol tags with multiple tags separated by commas.

```
# Graphite metric
servers.localhost.cpu.loadavg.10 23141 1459382400
# Template with pattern and tags
.host.resource.measurement* region=us-west,zone=1a
# Resulting point in InfluxDB line protocol
loadavg.10,host=localhost,resource=cpu,region=us-west,zone=1a value=23141 1459382400
```

Tags on a template will override any [input tags](#input-tags).

## Multiple Templates

One template may not work for all metrics.
For example, using multiple plugins with diamond will produce metrics in different formats.
In these cases, multiple templates can be defined with a prefix filter that must match a metric before a pattern will be applied.

Here is an example of the configuration to define a multiple templates.

```
templates = [
  "*.app env..service.measurement.field app=load_balancer",
  "stats.* .host.measurement.field",
  "server.*",
]
```

#### Filters

A filter can be added to the beginning of a template, before the pattern and separated by a space.

Filters have a similar format to patterns where the tokens are delineated by periods (`.`).
However, the tokens act like regular expressions on the corresponding part of the metric, either exact matching the token exactly or ignoring the part completely for wildcard (`*`) token.
When an incoming metric matches a filter, the pattern for that template is applied to the matching metric.

When there are multiple templates with filters, the template with the most specific filter is applied to matching points.

```
# Graphite metric
servers.localhost.cpu.loadavg.10 23141 1459382400
servers.host123.elasticsearch.cache_hits 100 1459382400
servers.host456.mysql.tx_count 10 1459382400
servers.host789.prod.mysql.tx_count 10 1459382400
# List of filters with matched Graphite metric paths
servers.localhost.*  -> matches "servers.localhost.cpu.loadavg"
servers.*.*.mysql    -> matches "servers.host789.prod.mysql.tx_count"
servers.*.mysql      -> matches "servers.host456.mysql.tx_count"
servers.*            -> matches "servers.host123.elasticsearch.cache_hits"
```

#### Base Template

If multiple templates with filters are defined, one base template can be defined to .
This template will apply to any metric that has not already matched a filter.

```
dev.http.requests.200
prod.myapp.errors.count
dev.db.queries.count
```

* `env.app.measurement*` would create
  * _measurement_=`requests.200` _tags_=`env=dev,app=http`
  * _measurement_= `errors.count` _tags_=`env=prod,app=myapp`
  * _measurement_=`queries.count` _tags_=`env=dev,app=db`

## Advanced Configuration Examples

The following configurations provide some additional examples which use the different template features explained above 

```
# Example configuration with complicated template setup
[[graphite]]
   enabled = true
   separator = "_"
   tags = ["region=us-east", "zone=1c"]
   templates = [
     # filter + template
     "*.app env.service.resource.measurement",

     # filter + template + extra tag
     "stats.* .host.measurement* region=us-west,agent=sensu",

     # filter + template with field key
     "stats.* .host.measurement.field",

     # default template. Ignore the first graphite component "servers"
     ".measurement*",
 ]
```

```
# Example with multiple Graphite listeners listening for UDP and TCP on different ports
[[graphite]]
  enabled = true
  bind-address = ":2003"
  protocol = "tcp"

[[graphite]]
  enabled = true
  bind-address = ":2004" # note different bind address
  protocol = "udp"
```



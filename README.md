# Math-Evaluate OpenChirp Service

## Overview
This is an OpenChirp service that evaluates a logical or mathematical
expression upon receiving updates to transducers referenced in the expression.

# Service Config
* `Expressions` - Required - Comma separated list of mathematical or logical expressions - `"(temp_c*1.8) + 32", temp_f > 100`
* `Output Topics` - Optional - Comma separated list of corresponding output topics - `temp_f, overtemp`
* `Options` - Optional - Comma separated list of optional behavior modifiers -
  `boolasvalue`
    - `boolasvalue` - Indicates that you want boolean values to be published
       as `0` or `1`.
       Without this modifier, booleans are published as `true` or `false`.
       This modifier currently does not allow the reverse direction, `0` or `1`
       being used as booleans.

# Expression Syntax
Expressions are evaluated by the [govaluate](https://github.com/Knetic/govaluate)
library.
Please see the library's [README.md](https://github.com/Knetic/govaluate/blob/master/README.md)
or the library's [MANUAL.md](https://github.com/Knetic/govaluate/blob/master/MANUAL.md)
for help with expressions.

# Developer Notes
* Transducer names with `-` must be used in the expression with the underscore,
  `_`, instead. This is because it is impossible to distinguish between the
  transducer name and the subtraction operator in the expression.

  Example:
  An expression of `"door-relay && motion-activated"` would be seen
  as `(door - relay) && (motion - activated)`, where it depends on the
  transducers `door`, `relay`, `motion`, and `activated`.
  Using `"door_relay && motion_activated"` will cause the service to depend on
  the transducers `door_relay`, `door-relay`, `motion_activated`, and
  `motion-activated`.
* It should be noted that this service assumes that transducers are only
  one level deep. This is because we must use them as variable names in an
  expressions. Since "/" maps to divide, we cannot express hierarchical
  transducer names.
* We allow one type of immediate loop, where the output topic is one of the
  input variable names. This allows for things like incrementing transducers
  on events.
  This kind of immediate loop is detected and handled removing the input
  variable(which matches the output topic) from the list of transducers
  capable of triggering a reevaluation.
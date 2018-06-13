# Math-Evaluate OpenChirp Service

## Overview
This is an OpenChirp service that evaluates a logical or mathematical
expression upon receiving updates to transducers referenced in the expression.

# Service Config
* `Expressions` - Required - Comma separated list of mathematical or logical expressions - "(temp_c*1.8) + 32", temp_f > 100
* `Output Topics` - Optional - Comma separated list of corresponding output topics - temp_f, overtemp
* `Options` - Optional - Comma separated list of optional behavior modifiers -
  `boolasvalue`
    - `boolasvalue` - Indicates that you want boolean values to be published
       as `0` or `1`.
       Without this modifier, booleans are published as `true` or `false`.
       This modifier currently does not allow the reverse direction, `0` or `1`
       being used as booleans.

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
# x Extension golang module placeholder

## Motivation

This project serves as an extension placeholder for a Go module, facilitating an initial setup before dynamic loading of extensions via the Go plugin system.

Consider a scenario where a main application requires extension through custom Go types, which are dynamically reloadable. 
Suppose the extension module is hosted at github.com/myorg/myapp/extension. 
This module defines several types and registers them with the [viant/pgin](https://github.com/viant/pgin) type Registry.

The main application, located at github.com/myorg/myapp, needs to access these types both through the Go plugin system for runtime extension and directly as a dependency for compile-time integration. 
This project is designed to support the direct dependency mechanism, ensuring seamless access to the extension types within the main application's ecosystem.


[Main Application] -> [Extension Module] -> [github.com/myorg/myapp/extension]


The main application go mod.  
```bash
module github.com/myorg/myapp

go 1.21
  require (
    github.com/viant/x v0.0.0
    github.com/viant/pgin v0.1.0
    github.com/viant/pgo v0.11.0
    ...
  )
    
replace github.com/viant/x => ../../myorg/myapp/x
```



The local transient extension ```go.mod```
```bash
module github.com/viant/x

go 1.21

require github.com/myapp/extension
```


The local transient extension ```init.go```
```bash
import _ "github.com/myorg/myapp/extension"
```



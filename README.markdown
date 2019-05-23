[![This project is considered experimental](https://img.shields.io/badge/Status-experimental-red.svg)](https://arp242.net/status/experimental)
[![Build Status](https://travis-ci.org/zgoat/zhttp.svg?branch=master)](https://travis-ci.org/zgoat/zhttp)
[![codecov](https://codecov.io/gh/zgoat/zhttp/branch/master/graph/badge.svg)](https://codecov.io/gh/zgoat/zhttp)
[![GoDoc](https://godoc.org/github.com/zgoat/zhttp?status.svg)](https://godoc.org/github.com/zgoat/zhttp)

Martin's HTTP package: It's not a "framework", but just a collection of
functions for building HTTP services.

Convention over configuration. Honestly, I'm not sure how useful this will be
for other people at this stage (or ever).

---

`zhttp.Wrap()` allows returning errors from HTTP endpoints:

    http.HandleFunc("/bar", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
      d, err := getData()
      if err != nil {
          return err
      }

      return zhttp.String("Hello, %s", d)
    }))

It's just more convenient than `http.Error(...)` followed by a `return`. The
`ErrFunc()` will be used to report returned errors (you can override it if you
need to).

Return helpers:

- `zhttp.String()`
- `zhttp.JSON()`
- `zhttp.Template()`
- `zhttp.SeeOther()`

--

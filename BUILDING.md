
# Building BitMED

Clone the repository and build the source:

```
git clone https://github.com/InsighterInc/bxmp.git
cd bxmp
make all
make test
```

Binaries are placed within `./build/bin`, most notably `geth` and `bootnode`. Either add this directory to your `$PATH` or copy those two bins into your PATH:

```sh
# assumes that /usr/local/bin is in your PATH
cp ./build/bin/geth ./build/bin/bootnode /usr/local/bin/
```

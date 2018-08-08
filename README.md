# BXMP

## Community Chat
[Gitter- BitMED](https://gitter.im/BitMED_BXM/Lobby) 

BitMED is an BitMED-based distributed ledger protocol with transaction/contract privacy and new consensus mechanisms.

BitMED is a fork of [Quorum](https://github.com/jpmorganchase/quorum) and is updated in line with go-ethereum releases.

Key enhancements over BXMP:

  * __Privacy__ - BitMED supports private transactions and private contracts through public/private state separation and utilising [Constellation](https://github.com/InsighterInc/constellation), a peer-to-peer encrypted message exchange for directed transfer of private data to network participants
  * __Alternative Consensus Mechanisms__ - with no need for POW/POS in a permissioned network, BitMED instead offers multiple consensus mechanisms that are more appropriate for consortium chains:
    * __Raft-based Consensus__ - a consensus model for faster blocktimes, transaction finality, and on-demand block creation
    * __Istanbul BFT__ - a PBFT-inspired consensus algorithm with transaction finality, by AMIS.
  * __Peer Permissioning__ - node/peer permissioning using smart contracts, ensuring only known parties can join the network
  * __Higher Performance__ - BitMED offers significantly higher performance than public geth


## Quickstart

The quickest way to get started with BitMED is using [VirtualBox](https://www.virtualbox.org/wiki/Downloads) and [Vagrant](https://www.vagrantup.com/downloads.html):

```sh
git clone https://github.com/InsighterInc/bxmp-examples
cd bxmp-examples
vagrant up
# (should take 5 or so minutes)
vagrant ssh
```

Now that you have a fully-functioning BitMED environment set up, let's run the 7-node cluster example. This will spin up several nodes with a mix of voters, block makers, and unprivileged nodes.

```sh
# (from within vagrant env, use `vagrant ssh` to enter)
ubuntu@ubuntu-xenial:~$ cd bxmp-examples/7nodes

$ ./raft-init.sh
# (output condensed for clarity)
[*] Cleaning up temporary data directories
[*] Configuring node 1
[*] Configuring node 2 as block maker and voter
[*] Configuring node 3
[*] Configuring node 4 as voter
[*] Configuring node 5 as voter
[*] Configuring node 6
[*] Configuring node 7

$ ./raft-start.sh
[*] Starting Constellation nodes
[*] Starting bootnode... waiting... done
[*] Starting node 1
[*] Starting node 2
[*] Starting node 3
[*] Starting node 4
[*] Starting node 5
[*] Starting node 6
[*] Starting node 7
[*] Unlocking account and sending first transaction
Contract transaction send: TransactionHash: 0xbfb7bfb97ba9bacbf768e67ac8ef05e4ac6960fc1eeb6ab38247db91448b8ec6 waiting to be mined...
true
```

We now have a 7-node BXMP cluster with a [private smart contract](https://github.com/InsighterInc/bxmp-examples/blob/master/examples/7nodes/script1.js) (SimpleStorage) sent from `node 1` "for" `node 7` (denoted by the public key passed via `privateFor: ["ROAZBWtSacxXQrOe3FGAqJDyJjFePR5ce4TSIzmJ0Bc="]` in the `sendTransaction` call).

Connect to any of the nodes and inspect them using the following commands:

```sh
$ geth attach ipc:qdata/dd1/geth.ipc
$ geth attach ipc:qdata/dd2/geth.ipc
...
$ geth attach ipc:qdata/dd7/geth.ipc


# e.g.

$ geth attach ipc:qdata/dd2/geth.ipc
Welcome to the Geth JavaScript console!

instance: Geth/v1.5.0-unstable/linux/go1.7.3
coinbase: 0xca843569e3427144cead5e4d5999a3d0ccf92b8e
at block: 679 (Tue, 15 Nov 2016 00:01:05 UTC)
 datadir: /home/ubuntu/bxmp-examples/7nodes/qdata/dd2
 modules: admin:1.0 debug:1.0 bxm:1.0 net:1.0 personal:1.0 quorum:1.0 rpc:1.0 txpool:1.0 web3:1.0

# let's look at the private txn created earlier:
> bxm.getTransaction("0xbfb7bfb97ba9bacbf768e67ac8ef05e4ac6960fc1eeb6ab38247db91448b8ec6")
{
  blockHash: "0xb6aec633ef1f79daddc071bec8a56b7099ab08ac9ff2dc2764ffb34d5a8d15f8",
  blockNumber: 1,
  from: "0xed9d02e382b34818e88b88a309c7fe71e65f419d",
  gas: 300000,
  gasPrice: 0,
  hash: "0xbfb7bfb97ba9bacbf768e67ac8ef05e4ac6960fc1eeb6ab38247db91448b8ec6",
  input: "0x9820c1a5869713757565daede6fcec57f3a6b45d659e59e72c98c531dcba9ed206fd0012c75ce72dc8b48cd079ac08536d3214b1a4043da8cea85be858b39c1d",
  nonce: 0,
  r: "0x226615349dc143a26852d91d2dff1e57b4259b576f675b06173e9972850089e7",
  s: "0x45d74765c5400c5c280dd6285a84032bdcb1de85a846e87b57e9e0cedad6c427",
  to: null,
  transactionIndex: 1,
  v: "0x25",
  value: 0
}
```

Note in particular the `v` field of "0x25" (37 in decimal) which marks this transaction as having a private payload (input).

## Demonstrating Privacy
Documentation detailing steps to demonstrate the privacy features of BitMED can be found in [bxmp-examples/7nodes/README](https://github.com/InsighterInc/bxmp-examples/tree/master/examples/7nodes/README.md).

## See also

* [BXMP](https://github.com/InsighterInc/bxmp): this repository
* [Constellation](https://github.com/InsighterInc/constellation): peer-to-peer encrypted message exchange for transaction privacy
* [Raft Consensus Documentation](raft/doc.md)
* [BXMP-examples](https://github.com/InsighterInc/bxmp-examples): example quorum clusters
* [BXMP-tools](https://github.com/InsighterInc/bxmp-tools): local cluster orchestration, and integration testing tool
* [BXMP Wiki](https://github.com/InsighterInc/bxmp/wiki)
* [bxm-web3js](https://github.com/InsighterInc/bxm-web3js) - an extension BXMP API

## Contributing

Thank you for your interest in contributing to BXMP!

BXMP is built on open source and we invite you to contribute enhancements. Upon review you will be required to complete a Contributor License Agreement (CLA) before we are able to merge. If you have any questions about the contribution process, please feel free to send an email to [info@bitmed.io](mailto:info@bitmed.io).

## License

The BXMP library (i.e. all code outside of the `cmd` directory) is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html), also
included in our repository in the `COPYING.LESSER` file.

The BXMP binaries (i.e. all code inside of the `cmd` directory) is licensed under the
[GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html), also included
in our repository in the `COPYING` file.

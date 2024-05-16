# aict

> Under development, not ready.

Another Internet Control Message Tunnel But Featuring Firewall Bypass

## Introduce

AICT is a tool that allows you to bypass firewalls and NATs by using ICMP packets. 

Originally created for emergency maintenance of servers located in my university network.

It's firstly written as plugin of [gost](https://github.com/go-gost/gost), but now it's a standalone tool.

## Why use aict?

Some firewalls match icmp packets by their seq and id, allowing icmp packet in only if it's a response to a packet sent by the client.

Also, once a packet is sent, the firewall will allow one more packet in, which make other icmp tunnel tools not working.

AICT uses a different approach, it maintains a seq and id pair queue in server side, and client side will watch out the traffic and send enough empty request packets to server side to keep the seq and id pair queue alive.

So once the server side wants to send a packet to client side, it can just pick a seq and id pair from the queue and send the packet with them.

## usage

turn off linux icmp reply firstly

```bash
echo 1 > /proc/sys/net/ipv4/icmp_echo_ignore_all
```

### client
```bash
./aict -c -r remote_ip
```

### server
```bash
./aict -s
```
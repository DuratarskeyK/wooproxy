#!/bin/sh

main_interface=$(awk '$2=="00000000" { print $1 }' /proc/net/route)
main_ip=$(curl -u api:$API_KEY -s $BASE_ADDR/server/$SERVER_ID/main_ip)
current_ip_file=$(mktemp)
new_ip_file=$(mktemp)
ip address show $main_interface | grep "inet " | grep -o "[0-9]*\.[0-9]*\.[0-9]*\.[0-9]*\/[0-9]*" | grep -v "$main_ip/" | sort > $current_ip_file
curl -su api:$API_KEY -s $BASE_ADDR/server/$SERVER_ID/ips?cidr | sort > $new_ip_file
changes=$(diff $current_ip_file -u $new_ip_file)

for i in $(echo "$changes" | grep '^+[^+]')
do
	ip address add `echo $i | tr -d +` dev $main_interface
	echo "Added $i"
done
for i in $(echo "$changes" | grep '^-[^-]')
do
	ip address del `echo $i | tr -d -` dev $main_interface
	echo "Removed $i"
done

rm $current_ip_file
rm $new_ip_file


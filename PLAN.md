Todo:

- Customizable port
- Per-process view — real-time only, no storage needed
- Alerting — sustained high CPU/memory/etc. for X duration triggers a notification (need to decide: browser notification, webhook, email?)
- Ability to inspect more than the last hour in the charts.

Testing:

for i in {1..8}; do yes > /dev/null & done

...

killall yes

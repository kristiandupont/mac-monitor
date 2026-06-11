Todo:

- It says "Apple M4, 10 cores" on the GPU card?
- Web: link to landing page
- Release on app store
- Check for updates

Future:

- Alerting — sustained high CPU/memory/etc. for X duration triggers a notification (need to decide: browser notification, webhook, email?)
- Ability to inspect more than the last hour in the charts.

Testing:

```bash
for i in {1..8}; do yes > /dev/null & done
```

...

```bash
killall yes
```

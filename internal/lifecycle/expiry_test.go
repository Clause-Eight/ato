package lifecycle

import (
	"strings"
	"testing"
)

// The code below is made up for these tests.

func expiry(p, text string) string {
	var out []string
	for _, e := range Find(p, []byte(text)) {
		out = append(out, e.Kind+":"+e.Target+":"+e.Engine)
	}
	return strings.Join(out, ",")
}

func TestFindExpiry(t *testing.T) {
	cases := []struct{ path, text, want string }{
		{"setup.py", "client.update_time_to_live(\n    TableName=\"sessions\",\n    TimeToLiveSpecification={\"Enabled\": True, \"AttributeName\": \"ttl\"},\n)\n", "retention:sessions:dynamodb"},
		{"setup.py", "client.update_time_to_live(\n    TableName=\"sessions\",\n    TimeToLiveSpecification={\"Enabled\": False, \"AttributeName\": \"ttl\"},\n)\n", ""},
		{"ttl.ts", "await ddb.send(new UpdateTimeToLiveCommand({\n  TableName: \"carts\",\n  TimeToLiveSpecification: { Enabled: true, AttributeName: \"expiresAt\" },\n}));\n", "retention:carts:dynamodb"},
		{"ttl.go", "_, err := client.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{\n\tTableName: aws.String(\"login_events\"),\n\tTimeToLiveSpecification: &types.TimeToLiveSpecification{Enabled: aws.Bool(true), AttributeName: aws.String(\"ttl\")},\n})\n", "retention:login_events:dynamodb"},
		{"Ttl.java", "ddb.updateTimeToLive(UpdateTimeToLiveRequest.builder().tableName(\"visits\").timeToLiveSpecification(spec).build());\n", "retention:visits:dynamodb"},
		{"stack.ts", "new dynamodb.Table(this, 'Sessions', {\n  tableName: 'sessions',\n  partitionKey: { name: 'id', type: dynamodb.AttributeType.STRING },\n  timeToLiveAttribute: 'expiresAt',\n});\n", "retention:sessions:dynamodb"},
		{"ttl.ts", "// UpdateTimeToLive is not used here\nconst tableName = config.table\n", ""},
		{"cache.py", "import redis\nr = redis.Redis()\nr.setex(f\"session:{sid}\", 3600, payload)\n", "retention:session:redis"},
		{"cache.py", "import redis\nr = redis.Redis()\nr.set(\"otp:\" + phone_hash, code, ex=300)\n", "retention:otp:redis"},
		{"cache.py", "import redis\nr = redis.Redis()\nr.set(\"profile:\" + uid, data)\n", ""},
		{"cache.py", "import redis\nr = redis.Redis()\nr.set(\"profile:\" + uid, data, ex=None)\n", ""},
		{"cache.js", "const redis = new Redis();\nawait redis.set(`cart:${id}`, body, 'EX', 86400);\n", "retention:cart:redis"},
		{"cache.js", "const client = createClient(); // redis\nawait client.set('reset_token:' + id, t, { EX: 900 });\n", "retention:reset_token:redis"},
		{"cache.js", "const redis = new Redis();\nawait redis.expire(key, 60);\n", ""},
		{"cache.go", "rdb := redis.NewClient(opts)\nrdb.Set(ctx, \"session:\"+id, v, 30*time.Minute)\n", "retention:session:redis"},
		{"cache.go", "rdb := redis.NewClient(opts)\nrdb.Set(ctx, \"profile:\"+id, v, 0)\n", ""},
		{"cache.go", "rdb := redis.NewClient(opts)\nrdb.Expire(ctx, \"visitor:\"+id, ttl)\n", "retention:visitor:redis"},
		{"Cache.java", "Jedis jedis = pool.getResource();\njedis.setex(\"session:\" + id, 1800, json);\n", "retention:session:redis"},
		{"Cache.java", "Jedis jedis = pool.getResource();\njedis.set(\"cart:\" + id, json, SetParams.setParams().ex(600));\n", "retention:cart:redis"},
		{"Cache.java", "redisTemplate.opsForValue().set(\"otp:\" + id, code, Duration.ofMinutes(5));\n", "retention:otp:redis"},
		{"Cache.php", "$redis = new \\Redis();\n$redis->setex('session:' . $id, 3600, $data);\n", "retention:session:redis"},
		{"Cache.php", "$client = new Predis\\Client();\n$client->set('cart:' . $id, $data, 'EX', 600);\n", "retention:cart:redis"},
		{"Cache.php", "$redis = new \\Redis();\n$redis->set('cart:' . $id, $data, ['ex' => 600]);\n", "retention:cart:redis"},
		{"cookies.js", "res.cookie.expire('session', 60)\n", ""},
		{"ilm/audit-logs-policy.json", "{\n  \"policy\": {\n    \"phases\": {\n      \"hot\": {\"actions\": {\"rollover\": {\"max_age\": \"7d\"}}},\n      \"delete\": {\"min_age\": \"90d\", \"actions\": {\"delete\": {}}}\n    }\n  }\n}\n", "retention:audit-logs:search"},
		{"ism/visits.json", "{\"policy\": {\"states\": [{\"name\": \"hot\", \"actions\": [{\"delete\": {}}]}], \"ism_template\": [{\"index_patterns\": [\"visits-*\"]}]}}\n", "retention:visits:search"},
		{"ilm/hot-only.json", "{\"policy\": {\"phases\": {\"hot\": {\"actions\": {\"rollover\": {\"max_age\": \"7d\"}}}}}}\n", ""},
		{"package.json", "{\"scripts\": {\"delete\": {\"x\": 1}}}\n", ""},
		{"ilm.py", "es.ilm.put_lifecycle(name=\"patient-events-policy\", policy={\n    \"phases\": {\"delete\": {\"min_age\": \"30d\", \"actions\": {\"delete\": {}}}}\n})\n", "retention:patient-events:search"},
		{"ism.ts", "await client.transport.request({ method: 'PUT', path: '/_plugins/_ism/policies/sessions', body: {\n  policy: { states: [{ name: 'hot', actions: [{ delete: {} }] }] },\n} })\n", "retention:sessions:search"},
		{"routes.ts", "const routes = { delete: { path: '/x' }, states: [] }\n", ""},
	}
	for _, c := range cases {
		if got := expiry(c.path, c.text); got != c.want {
			t.Errorf("%s %q: got %q, want %q", c.path, c.text, got, c.want)
		}
	}
}

func TestFindExpiryOnly(t *testing.T) {
	if got := FindExpiry("q.sql", []byte("DELETE FROM customers WHERE id = $1;\n")); len(got) != 0 {
		t.Errorf("erasure is not expiry: %+v", got)
	}
	code := "import redis\nr.setex(\"session:\" + sid, 60, v)\nCustomer.objects.filter(pk=pk).delete()\n"
	got := FindExpiry("cache.py", []byte(code))
	if len(got) != 1 || got[0].Engine != Redis || got[0].Location.Line != 2 {
		t.Errorf("got %+v", got)
	}
	if got := FindExpiry("notes.txt", []byte(code)); got != nil {
		t.Errorf("not a candidate: %+v", got)
	}
}

func TestAssessCountsStoreExpiry(t *testing.T) {
	ev := Find("cache.py", []byte("import redis\nr.setex(\"session:\" + sid, 60, v)\n"))
	if st := Assess([]string{"sessions"}, ev)["sessions"]; len(st.Retention) != 1 {
		t.Errorf("a Redis key expiry is a retention limit for sessions: %+v", st)
	}
}

package lifecycle

import (
	"strings"
	"testing"
)

func kinds(p, text string) string {
	var out []string
	for _, e := range Find(p, []byte(text)) {
		s := e.Kind + ":" + e.Target
		if e.Parent != "" {
			s += "<" + e.Parent
		}
		out = append(out, s)
	}
	return strings.Join(out, ",")
}

func TestFind(t *testing.T) {
	cases := []struct{ path, text, want string }{
		{"q.sql", "DELETE FROM patients WHERE id = $1;", "erasure:patients"},
		{"q.sql", "DELETE FROM public.\"audit_log\" WHERE created_at < now() - interval '90 days';", "retention:audit_log"},
		{"q.sql", "UPDATE customers SET email = NULL, phone = NULL WHERE id = $1;", "erasure:customers"},
		{"q.sql", "UPDATE customers SET plan = 'pro' WHERE id = $1;", ""},
		{"views.py", "    Patient.objects.filter(pk=pk).delete()", "erasure:Patient"},
		{"jobs.py", "    Session.objects.filter(created_at__lt=cutoff).delete()", "retention:Session"},
		{"repo.py", "    session.execute(delete(Customer).where(Customer.id == cid))", "erasure:Customer"},
		{"user.rb", "    User.find(id).destroy", "erasure:User"},
		{"job.rb", "    Visit.where(\"created_at < ?\", 30.days.ago).delete_all", "retention:Visit"},
		{"repo.go", "\treturn db.Delete(&Customer{}, id).Error", "erasure:Customer"},
		{"job.go", "\tdb.Where(\"created_at < ?\", time.Now().Add(-ttl)).Delete(&models.Event{})", "retention:Event"},
		{"Svc.java", "        patientRepository.deleteById(id);", "erasure:patient"},
		{"svc.ts", "  await prisma.customer.delete({ where: { id } })", "erasure:customer"},
		{"svc.ts", "  await prisma.loginEvent.deleteMany({ where: { createdAt: { lt: cutoff } } })", "retention:loginEvent"},
		{"C.php", "        Customer::destroy($id);", "erasure:Customer"},
		{"gdpr.py", "def anonymise_customer(customer_id):", "erasure:customer"},
		{"cron.py", "def purge_old_sessions():", "retention:sessions"},
		{"init.js", "db.sessions.createIndex({ createdAt: 1 }, { expireAfterSeconds: 86400 })", "retention:sessions"},
		{"p.sql", "ALTER TABLE events DETACH PARTITION events_2024_01;", "retention:events"},
		{"s.sql", "CREATE TABLE visits (\n  patient_id bigint REFERENCES patients(id) ON DELETE CASCADE\n);", "cascade:visits<patients"},
		{"models.py", "class Visit(models.Model):\n    patient = models.ForeignKey(Patient, on_delete=models.CASCADE)\n", "cascade:Visit<Patient"},
		{"user.rb", "class User < ApplicationRecord\n  has_many :addresses, dependent: :destroy\nend\n", "cascade:addresses<User"},
		{"notes.py", "# we never delete anything\nx = obj.delete_flag\n", ""},
		{"svc.js", "  await Customer.deleteOne({ _id: id });", "erasure:Customer"},
		{"job.js", "  await Session.deleteMany({ createdAt: { $lt: cutoff } });", "retention:Session"},
		{"user.js", "userSchema.index({ createdAt: 1 }, { expireAfterSeconds: 2592000 });", "retention:user"},
		{"svc.ts", "  await dataSource.getRepository(Patient).delete({ id });", "erasure:Patient"},
		{"svc.ts", "  await this.manager.delete(Visit, { id });", "erasure:Visit"},
		{"order.ts", "export class Order {\n  @ManyToOne(() => Customer, (c) => c.orders, { onDelete: 'CASCADE' })\n  customer: Customer;\n}\n", "cascade:Order<Customer"},
		{"Repo.php", "        $qb->delete(Patient::class, 'p')->where('p.id = :id');", "erasure:Patient"},
		{"Repo.php", "        $this->em->createQuery('DELETE App\\Entity\\Visit v WHERE v.id = :id');", "erasure:Visit"},
		{"Svc.php", "        DB::table('customers')->where('id', $id)->delete();", "erasure:customers"},
		{"m.php", "Schema::create('visits', function (Blueprint $table) {\n    $table->foreignId('patient_id')->constrained()->cascadeOnDelete();\n    $table->foreign('doctor_id')->references('id')->on('doctors')->onDelete('cascade');\n});\n", "cascade:visits<patient,cascade:visits<doctors"},
	}
	for _, c := range cases {
		if got := kinds(c.path, c.text); got != c.want {
			t.Errorf("%s %q: got %q, want %q", c.path, c.text, got, c.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{"patients": "patient", "Patient": "patient", "user_profiles": "userprofile", "UserProfile": "userprofile", "addresses": "address", "address": "address", "categories": "category", "status": "statu", "boxes": "box"} {
		if got := Normalize(in); got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}

func TestAssessFollowsCascades(t *testing.T) {
	ev := append(Find("a.sql", []byte("CREATE TABLE visits (\n  patient_id bigint REFERENCES patients(id) ON DELETE CASCADE\n);\nCREATE TABLE notes (\n  visit_id bigint REFERENCES visits(id) ON DELETE CASCADE\n);\n")),
		Find("svc.py", []byte("    Patient.objects.filter(pk=pk).delete()\n    AuditLog.objects.filter(created_at__lt=c).delete()\n"))...)
	got := Assess([]string{"patients", "visits", "notes", "audit_logs", "invoices"}, ev)
	for store, want := range map[string][2]bool{"patients": {true, false}, "visits": {true, false}, "notes": {true, false}, "audit_logs": {false, true}, "invoices": {false, false}} {
		st := got[store]
		if (len(st.Erasure) > 0) != want[0] || (len(st.Retention) > 0) != want[1] {
			t.Errorf("%s: erasure %v retention %v, want %v", store, st.Erasure, st.Retention, want)
		}
	}
}

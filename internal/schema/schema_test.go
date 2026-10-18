package schema

import (
	"strings"
	"testing"
)

func names(t *testing.T, path, text string) []string {
	t.Helper()
	var out []string
	for _, f := range Fields(path, []byte(text)) {
		if f.Location.Path != path || f.Location.Line < 1 || f.Format == "" {
			t.Fatalf("bad field %+v", f)
		}
		out = append(out, f.Container+"."+f.Name)
	}
	return out
}

func expect(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestSQL(t *testing.T) {
	got := names(t, "db/migrations/001_patients.sql", `
CREATE TABLE IF NOT EXISTS public."patients" (
  id bigserial PRIMARY KEY,
  first_name varchar(100) NOT NULL, -- given name
  "blood_type" char(3),
  amount numeric(10, 2) DEFAULT 0,
  CONSTRAINT patients_name_unique UNIQUE (first_name),
  PRIMARY KEY (id)
);
ALTER TABLE patients ADD COLUMN religion text;
alter table patients rename column religion to faith;
`)
	expect(t, got, "patients.id", "patients.first_name", "patients.blood_type", "patients.amount", "patients.religion", "patients.faith")
}

func TestORMs(t *testing.T) {
	expect(t, names(t, "app/models.py", `
from django.db import models

class Patient(models.Model):
    first_name = models.CharField(max_length=100)
    diagnosis = models.TextField()
    def __str__(self):
        return self.first_name
`), "Patient.first_name", "Patient.diagnosis")

	expect(t, names(t, "app/db.py", `
from sqlalchemy.orm import Mapped, mapped_column
class Member(Base):
    __tablename__ = "members"
    id: Mapped[int] = mapped_column(primary_key=True)
    union_member: Mapped[bool]
    email = Column(String)
`), "Member.id", "Member.union_member", "Member.email")

	expect(t, names(t, "db/migrate/20260101_create_users.rb", `
class CreateUsers < ActiveRecord::Migration[7.1]
  def change
    create_table :users do |t|
      t.string :last_name
      t.date :date_of_birth
    end
    add_column :users, :ethnicity, :string
  end
end
`), "users.last_name", "users.date_of_birth", "users.ethnicity")

	expect(t, names(t, "internal/store/user.go", "package store\n\ntype User struct {\n\tgorm.Model\n\tEmail string `gorm:\"uniqueIndex\"`\n\tHealthNotes string `gorm:\"type:text\" json:\"health_notes\"`\n\tcount int\n}\n"), "User.Email", "User.HealthNotes")

	expect(t, names(t, "src/main/java/app/Patient.java", `
@Entity
@Table(name = "patients")
public class Patient {
    @Id
    private Long id;
    @Column(name = "blood_group")
    private String bloodGroup;
    private String diagnosis;
}
`), "Patient.id", "Patient.blood_group", "Patient.diagnosis")

	expect(t, names(t, "prisma/schema.prisma", `
model User {
  id        Int      @id @default(autoincrement())
  email     String   @unique
  religion  String?
  @@index([email])
}
`), "User.id", "User.email", "User.religion")

	expect(t, names(t, "proto/user.proto", `
syntax = "proto3";
message User {
  string first_name = 1;
  repeated string allergies = 2;
  map<string, string> labels = 3;
}
`), "User.first_name", "User.allergies", "User.labels")
}

func TestOpenAPIAndJSONSchema(t *testing.T) {
	expect(t, names(t, "api/openapi.yaml", `
openapi: 3.0.3
components:
  schemas:
    Patient:
      type: object
      properties:
        firstName:
          type: string
        diagnosis:
          type: string
          description: "free text"
`), "Patient.firstName", "Patient.diagnosis")

	expect(t, names(t, "schemas/user.schema.json", `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "email": { "type": "string" },
    "religion": { "type": "string" }
  }
}`), ".email", ".religion")
}

func TestDBT(t *testing.T) {
	expect(t, names(t, "models/schema.yml", `
version: 2
models:
  - name: dim_patients
    description: patients
    columns:
      - name: patient_id
        tests: [unique]
      - name: diagnosis_code
`), "dim_patients.patient_id", "dim_patients.diagnosis_code")
}

func TestUnrelatedFilesGiveNothing(t *testing.T) {
	for path, text := range map[string]string{
		"main.py":      "def handler(event):\n    x = 1\n",
		"config.yaml":  "server:\n  port: 8080\n",
		"package.json": `{"name": "app", "version": "1.0.0"}`,
		"README.md":    "CREATE TABLE users (email text);",
		"cmd/main.go":  "package main\n\ntype Config struct {\n\tPort int\n}\n",
	} {
		if got := Fields(path, []byte(text)); len(got) != 0 {
			t.Errorf("%s: got %+v", path, got)
		}
	}
}

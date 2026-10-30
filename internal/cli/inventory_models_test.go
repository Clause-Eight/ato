package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The models and Terraform here are made up and hold field names only.
func TestInventoryReadsJavaScriptAndPHPModelsAndModuleCalls(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"src/models/User.ts":      "import { Entity, Column } from \"typeorm\";\n\n@Entity()\nexport class User {\n  @Column()\n  email: string;\n}\n",
		"models/patient.js":       "const mongoose = require(\"mongoose\");\nconst patientSchema = new mongoose.Schema({\n  phone: { type: String },\n  diagnosis: String,\n});\nmodule.exports = mongoose.model(\"Patient\", patientSchema);\n",
		"src/Entity/Customer.php": "<?php\nuse Doctrine\\ORM\\Mapping as ORM;\n\n#[ORM\\Entity]\nclass Customer\n{\n    #[ORM\\Column]\n    private string $lastName;\n\n    /** @ORM\\Column(type=\"string\") */\n    private $iban;\n}\n",
		"database/migrations/2026_01_01_create_members_table.php": "<?php\nuse Illuminate\\Database\\Schema\\Blueprint;\nuse Illuminate\\Support\\Facades\\Schema;\n\nSchema::create('members', function (Blueprint $table) {\n    $table->string('religion');\n});\n",
		"infra/main.tf": "module \"patients_db\" {\n  source            = \"terraform-aws-modules/rds/aws\"\n  storage_encrypted = true\n}\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errOut := run("inventory", "--no-git", dir)
	if code != ExitClean {
		t.Fatalf("inventory: exit %d %s\n%s", code, errOut, out)
	}
	for _, want := range []string{
		"email,User,contact,,typeorm,src/models/User.ts,6",
		"phone,Patient,contact,,mongoose,models/patient.js,3",
		"diagnosis,Patient,special-category,health,mongoose,models/patient.js,4",
		"lastName,Customer,name,,doctrine,src/Entity/Customer.php,8",
		"iban,Customer,financial,,doctrine,src/Entity/Customer.php,11",
		"religion,members,special-category,religious-or-philosophical-beliefs,laravel,database/migrations/2026_01_01_create_members_table.php,6",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inventory misses %q:\n%s", want, out)
		}
	}

	code, out, _ = run("inventory", "--no-git", "--format", "json", dir)
	if code != ExitClean {
		t.Fatalf("inventory json: exit %d\n%s", code, out)
	}
	var doc struct {
		Stores []struct {
			Type, Name, Service string
			Encrypted           string `json:"encrypted"`
			PersonalData        bool   `json:"personal_data"`
			Module              *struct {
				Source   string
				Settings []string
			}
		}
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Stores) != 1 {
		t.Fatalf("stores: %+v", doc.Stores)
	}
	s := doc.Stores[0]
	if s.Type != "module" || s.Name != "patients_db" || s.Service != "database" || s.Encrypted != "unknown" || !s.PersonalData ||
		s.Module == nil || s.Module.Source != "terraform-aws-modules/rds/aws" || strings.Join(s.Module.Settings, ",") != "storage_encrypted" {
		t.Errorf("module call: %+v", s)
	}

	if _, out, _ := run("scan", "--no-git", "--format", "json", dir); strings.Contains(out, "module.patients_db") || !strings.Contains(out, `"masked": "Patient.diagnosis"`) {
		t.Errorf("a module call is listed in the inventory, not reported as a finding:\n%s", out)
	}
}

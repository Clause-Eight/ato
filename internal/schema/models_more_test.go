package schema

import "testing"

// The models below are made up and hold field names only.

func TestTypeORM(t *testing.T) {
	cases := []struct {
		name, path, text string
		want             []string
	}{
		{"decorators on their own lines", "src/entity/User.ts", `
import { Entity, Column, PrimaryGeneratedColumn } from "typeorm";

@Entity()
export class User {
    @PrimaryGeneratedColumn()
    id: number;

    @Column()
    firstName: string;

    @Column({ type: "varchar", length: 200, nullable: true })
    @Index()
    public email?: string;

    @Column({ name: "blood_type" })
    bloodType!: string;

    @ManyToOne(() => Team)
    team: Team;

    // @Column() oldField: string;

    get displayName() { return this.firstName; }
}
`, []string{"User.id", "User.firstName", "User.email", "User.blood_type"}},
		{"decorator and property on one line", "src/patient.entity.ts", `
@Entity("patients")
export class Patient extends BaseEntity {
  @PrimaryColumn() nationalId: string;
  @Column("text") diagnosis: string;
  @CreateDateColumn() createdAt: Date;
}
`, []string{"Patient.nationalId", "Patient.diagnosis", "Patient.createdAt"}},
		{"plain JavaScript with Babel decorators", "models/member.js", `
@Entity()
class Member {
  @Column()
  religion;
}
`, []string{"Member.religion"}},
		{"no entity in the file", "src/dto.ts", "export class Dto {\n  @IsString()\n  email: string;\n}\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { expect(t, names(t, c.path, c.text), c.want...) })
	}
}

func TestMongoose(t *testing.T) {
	cases := []struct {
		name, path, text string
		want             []string
	}{
		{"schema registered as a model", "models/user.js", `
const mongoose = require("mongoose");
const { Schema } = mongoose;

const userSchema = new Schema({
  email: String,
  phone: { type: String, required: true },
  address: {
    street: String,
    city: { type: String },
  },
  'dateOfBirth': Date,
  allergies: [{ name: String, severity: Number }],
  tags: [String],
  // passport: String,
}, { timestamps: true });

module.exports = mongoose.model("Customer", userSchema);
`, []string{
			"Customer.email", "Customer.phone", "Customer.address", "Customer.address.street", "Customer.address.city",
			"Customer.dateOfBirth", "Customer.allergies", "Customer.allergies.name", "Customer.allergies.severity", "Customer.tags",
		}},
		{"TypeScript generic, no model call", "src/models/patient.ts", `
import mongoose, { Schema } from "mongoose";
export const PatientSchema = new mongoose.Schema<IPatient>({
  lastName: { type: String },
  diagnosis: String,
});
`, []string{"Patient.lastName", "Patient.diagnosis"}},
		{"schema written inside the model call", "models/visit.mjs", `
import mongoose from "mongoose";
export default mongoose.model("Visit", new mongoose.Schema({ ipAddress: String }));
`, []string{"Visit.ipAddress"}},
		{"not a Mongoose file", "src/form.js", "const schema = new Schema({ email: String });\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { expect(t, names(t, c.path, c.text), c.want...) })
	}
}

func TestDoctrine(t *testing.T) {
	cases := []struct {
		name, path, text string
		want             []string
	}{
		{"attributes", "src/Entity/Patient.php", `<?php
namespace App\Entity;

use Doctrine\ORM\Mapping as ORM;

#[ORM\Entity]
#[ORM\Table(name: 'patients')]
class Patient
{
    #[ORM\Id, ORM\Column(type: 'integer')]
    #[ORM\GeneratedValue]
    private ?int $id = null;

    #[ORM\Column(length: 100)]
    private string $firstName;

    #[ORM\Column(name: 'blood_group', nullable: true)]
    private ?string $bloodGroup = null;

    #[ORM\ManyToOne(targetEntity: Clinic::class)]
    #[ORM\JoinColumn(name: 'clinic_id')]
    private Clinic $clinic;

    public function __construct(
        #[ORM\Column(type: 'text')]
        private readonly string $diagnosis,
    ) {}
}
`, []string{"Patient.id", "Patient.firstName", "Patient.blood_group", "Patient.diagnosis"}},
		{"docblock annotations", "src/Entity/Customer.php", `<?php
use Doctrine\ORM\Mapping as ORM;

/**
 * @ORM\Entity
 * @ORM\Table(name="customers")
 */
class Customer
{
    /**
     * @ORM\Column(type="string", name="e_mail")
     */
    protected $email;

    /** @ORM\Column(type="string") */
    private $iban;

    /** @ORM\OneToMany(targetEntity="Order", mappedBy="customer") */
    private $orders;
}
`, []string{"Customer.e_mail", "Customer.iban"}},
		{"not Doctrine", "src/Service.php", "<?php\nclass Service {\n    private $email;\n}\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { expect(t, names(t, c.path, c.text), c.want...) })
	}
}

func TestLaravel(t *testing.T) {
	cases := []struct {
		name, path, text string
		want             []string
	}{
		{"create and alter", "database/migrations/2026_01_01_000000_create_users_table.php", `<?php
use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('users', function (Blueprint $table) {
            $table->id();
            $table->string('email')->unique();
            $table->string("last_name", 100)->nullable();
            $table->date('date_of_birth');
            $table->foreignId('team_id')->constrained()->cascadeOnDelete();
            // $table->string('passport');
            $table->index('email');
            $table->timestamps();
        });
        Schema::table('patients', function (Blueprint $t) {
            $t->text('diagnosis')->after('id');
            $t->renameColumn('religion', 'faith');
            $t->dropColumn('notes');
        });
    }
};
`, []string{"users.email", "users.last_name", "users.date_of_birth", "users.team_id", "patients.diagnosis", "patients.faith"}},
		{"not a migration", "app/Http/Controller.php", "<?php\n$table->string('email');\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { expect(t, names(t, c.path, c.text), c.want...) })
	}
}

// FuzzReaders checks that no input makes the JavaScript and PHP readers fail or hang.
func FuzzReaders(f *testing.F) {
	for _, s := range []string{
		"@Entity() class A { @Column({ name: 'x' }) y: string }",
		"mongoose new Schema({ a: { b: [{ c: String }] } }) model('A', s)",
		"<?php Doctrine\\ORM #[ORM\\Column(name: 'x')] private $y; /** @ORM\\Column */ var $z;",
		"<?php Schema::create('t', function (Blueprint $t) { $t->string('a'); $t->renameColumn('b'",
		"@Entity @Column(", "mongoose new Schema({", "new Schema({ a: [", "#[ORM\\Column(",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, p := range []string{"a.ts", "a.js", "a.php"} {
			for _, fd := range Fields(p, []byte(s)) {
				if fd.Location.Line < 1 || fd.Location.Column < 1 {
					t.Fatalf("bad position %+v", fd)
				}
			}
		}
	})
}

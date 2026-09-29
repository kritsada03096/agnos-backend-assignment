CREATE TABLE hospitals (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    name TEXT NOT NULL,
    api_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE staff (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    hospital_id BIGINT NOT NULL REFERENCES hospitals(id),
    username VARCHAR(64) NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (hospital_id, username)
);

CREATE TABLE patients (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    hospital_id BIGINT NOT NULL REFERENCES hospitals(id),
    patient_hn VARCHAR(100) NOT NULL CHECK (length(trim(patient_hn)) > 0),
    national_id VARCHAR(254),
    passport_id VARCHAR(254),
    first_name_th VARCHAR(254),
    middle_name_th VARCHAR(254),
    last_name_th VARCHAR(254),
    first_name_en VARCHAR(254),
    middle_name_en VARCHAR(254),
    last_name_en VARCHAR(254),
    date_of_birth DATE,
    phone_number VARCHAR(254),
    email VARCHAR(254),
    gender CHAR(1) CHECK (gender IN ('M', 'F')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_synced_at TIMESTAMPTZ,
    UNIQUE (hospital_id, patient_hn)
);
CREATE INDEX patients_hospital_national_id ON patients(hospital_id, national_id);
CREATE INDEX patients_hospital_passport_id ON patients(hospital_id, passport_id);
CREATE INDEX patients_hospital_id ON patients(hospital_id, id);

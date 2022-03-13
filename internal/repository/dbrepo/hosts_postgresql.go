package dbrepo

import (
	"context"
	"log"
	"time"

	"github.com/tsawler/vigilate/internal/models"
)

// Inserts a host into the database
func (m *postgresDBRepo) InsertHost(h models.Host) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `
		INSERT INTO hosts (host_name, canonical_name, url, ip, ipv6, location, os, active, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) returning id
	`

	var newID int

	err := m.DB.QueryRowContext(ctx, stmt,
		h.HostName,
		h.CanonicalName,
		h.URL,
		h.IP,
		h.IPV6,
		h.Location,
		h.OS,
		h.Active,
		time.Now(),
		time.Now(),
	).Scan(&newID)

	if err != nil {
		log.Println(err)
		return newID, err
	}

	// add host services and set to inactive
	query := `SELECT id FROM services`
	serviceRows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		log.Println(err)
		return 0, err
	}
	defer serviceRows.Close()
	for serviceRows.Next() {
		var svcID int
		err := serviceRows.Scan(&svcID)
		if err != nil {
			log.Println(err)
			return newID, err
		}

		stmt = `
		INSERT INTO host_services (
			host_id, 
			service_id, 
			active, 
			schedule_number, 
			schedule_unit, 
			status, 
			created_at, 
			updated_at
		) VALUES (
			$1, $2, 0, 3, 'm', 'pending', $3, $4
		)
	`
		_, err = m.DB.ExecContext(ctx, stmt, newID, svcID, time.Now(), time.Now())
		if err != nil {
			return newID, err
		}
	}

	return newID, nil
}

func (m *postgresDBRepo) GetHostById(id int) (models.Host, error) {
	var h models.Host

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `SELECT * FROM hosts WHERE id = $1`

	row := m.DB.QueryRowContext(ctx, query, id)

	err := row.Scan(
		&h.ID,
		&h.HostName,
		&h.CanonicalName,
		&h.URL,
		&h.IP,
		&h.IPV6,
		&h.Location,
		&h.OS,
		&h.Active,
		&h.CreatedAt,
		&h.UpdatedAt,
	)
	if err != nil {
		return h, err
	}

	// get all services for host
	query = `
		SELECT 
			hs.id, 
			hs.host_id, 
			hs.service_id, 
			hs.active, 
			hs.schedule_number, 
			hs.schedule_unit, 
			hs.last_check, 
			hs.status, 
			hs.created_at, 
			hs.updated_at, 
			s.id, 
			s.service_name, 
			s.active,  
			s.icon, 
			s.created_at, 
			s.updated_at
		FROM host_services hs
			LEFT JOIN services s ON (s.id = hs.service_id)
		WHERE host_id = $1 
		ORDER BY s.service_name;
	`

	rows, err := m.DB.QueryContext(ctx, query, h.ID)
	if err != nil {
		log.Println(err)
		return h, err
	}
	defer rows.Close()

	var hostServices []models.HostService
	for rows.Next() {
		var hs models.HostService
		err := rows.Scan(
			&hs.ID,
			&hs.HostID,
			&hs.ServiceID,
			&hs.Active,
			&hs.ScheduleNumber,
			&hs.ScheduleUnit,
			&hs.LastCheck,
			&hs.Status,
			&hs.CreatedAt,
			&hs.UpdatedAt,
			&hs.Service.ID,
			&hs.Service.ServiceName,
			&hs.Service.Active,
			&hs.Service.Icon,
			&hs.Service.CreatedAt,
			&hs.Service.UpdatedAt,
		)
		if err != nil {
			log.Println(err)
			return h, err
		}

		hostServices = append(hostServices, hs)
	}

	h.HostServices = hostServices

	return h, nil
}

func (m *postgresDBRepo) UpdateHost(h models.Host) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		UPDATE hosts 
			SET host_name = $2, 
			canonical_name = $3, 
			url = $4, 
			ip = $5, 
			ipv6 = $6, 
			location = $7, 
			os = $8, 
			active = $9, 
			updated_at = $10 
			WHERE id = $1
			`

	_, err := m.DB.ExecContext(ctx, query,
		h.ID,
		h.HostName,
		h.CanonicalName,
		h.URL,
		h.IP,
		h.IPV6,
		h.Location,
		h.OS,
		h.Active,
		time.Now(),
	)

	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (m *postgresDBRepo) GetAllServiceStatusCounts() (int, int, int, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
	SELECT 
		(SELECT count(id) FROM host_services WHERE active = 1 AND status = 'pending') AS pending,
		(SELECT count(id) FROM host_services WHERE active = 1 AND status = 'healthy') AS healthy,
		(SELECT count(id) FROM host_services WHERE active = 1 AND status = 'warning') AS warning,
		(SELECT count(id) FROM host_services WHERE active = 1 AND status = 'problem') AS problem;
	`

	var pending, healthy, warning, problem int

	row := m.DB.QueryRowContext(ctx, query)
	err := row.Scan(
		&pending,
		&healthy,
		&warning,
		&problem,
	)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return pending, healthy, warning, problem, nil
}

func (m *postgresDBRepo) AllHosts() ([]models.Host, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `SELECT * FROM hosts ORDER BY host_name`

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []models.Host

	for rows.Next() {
		var h models.Host
		err = rows.Scan(
			&h.ID,
			&h.HostName,
			&h.CanonicalName,
			&h.URL,
			&h.IP,
			&h.IPV6,
			&h.Location,
			&h.OS,
			&h.Active,
			&h.CreatedAt,
			&h.UpdatedAt,
		)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		serviceQuery := `
			SELECT 
				hs.id, 
				hs.host_id, 
				hs.service_id, 
				hs.active, 
				hs.schedule_number, 
				hs.schedule_unit, 
				hs.last_check, 
				hs.status, 
				hs.last_message, 
				hs.created_at, 
				hs.updated_at, 
				s.id, 
				s.service_name, 
				s.active,  
				s.icon, 
				s.created_at, 
				s.updated_at
			FROM host_services hs
				LEFT JOIN services s ON (s.id = hs.service_id)
			WHERE host_id = $1
		`
		serviceRows, err := m.DB.QueryContext(ctx, serviceQuery, h.ID)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		var hostServices []models.HostService
		for serviceRows.Next() {
			var hs models.HostService
			err = serviceRows.Scan(
				&hs.ID,
				&hs.HostID,
				&hs.ServiceID,
				&hs.Active,
				&hs.ScheduleNumber,
				&hs.ScheduleUnit,
				&hs.LastCheck,
				&hs.Status,
				&hs.LastMessage,
				&hs.CreatedAt,
				&hs.UpdatedAt,
				&hs.Service.ID,
				&hs.Service.ServiceName,
				&hs.Service.Active,
				&hs.Service.Icon,
				&hs.Service.CreatedAt,
				&hs.Service.UpdatedAt,
			)
			if err != nil {
				log.Println(err)
				return nil, err
			}
			hostServices = append(hostServices, hs)
			serviceRows.Close()
		}
		h.HostServices = hostServices
		hosts = append(hosts, h)
	}

	if err = rows.Err(); err != nil {
		log.Println(err)
		return nil, err
	}

	return hosts, nil
}

// UpdateHostServiceStatus updates the active status of a host service
func (m *postgresDBRepo) UpdateHostServiceStatus(hostID, serviceID, active int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `
		UPDATE host_services SET active = $1 
		WHERE host_id = $2 AND service_id = $3
	`

	_, err := m.DB.ExecContext(ctx, stmt, active, hostID, serviceID)
	if err != nil {
		return err
	}

	return nil
}

// UpdateHostService updates a host service
func (m *postgresDBRepo) UpdateHostService(hs models.HostService) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `
		UPDATE host_services SET 
			host_id = $1, service_id = $2, active = $3, 
			schedule_number = $4, schedule_unit = $5, 
			last_check = $6, status = $7, last_message = $8, updated_at = $9 
		WHERE id = $10
	`

	_, err := m.DB.ExecContext(ctx, stmt,
		&hs.HostID,
		&hs.ServiceID,
		&hs.Active,
		&hs.ScheduleNumber,
		&hs.ScheduleUnit,
		&hs.LastCheck,
		&hs.Status,
		&hs.LastMessage,
		&hs.UpdatedAt,
		&hs.ID,
	)
	if err != nil {
		return err
	}

	return nil
}

func (m *postgresDBRepo) GetServicesByStatus(status string) ([]models.HostService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		SELECT 
			hs.id, 
			hs.host_id, 
			hs.service_id, 
			hs.active, 
			hs.schedule_number, 
			hs.schedule_unit, 
			hs.last_check, 
			hs.status, 
			hs.last_message, 
			hs.created_at, 
			hs.updated_at, 
			h.host_name, 
			s.service_name 
		FROM host_services AS hs 
			LEFT JOIN hosts AS h ON (hs.host_id = h.id) 
			LEFT JOIN services AS s ON (hs.service_id = s.id) 
		WHERE status = $1 AND hs.active = 1 
		ORDER BY host_name, service_name;
	`

	var services []models.HostService

	rows, err := m.DB.QueryContext(ctx, query, status)
	if err != nil {
		log.Println(err)
		return services, err
	}
	defer rows.Close()

	for rows.Next() {
		var h models.HostService

		err := rows.Scan(
			&h.ID,
			&h.HostID,
			&h.ServiceID,
			&h.Active,
			&h.ScheduleNumber,
			&h.ScheduleUnit,
			&h.LastCheck,
			&h.Status,
			&h.LastMessage,
			&h.CreatedAt,
			&h.UpdatedAt,
			&h.HostName,
			&h.Service.ServiceName,
		)

		if err != nil {
			return nil, err
		}

		services = append(services, h)
	}

	return services, nil
}

func (m *postgresDBRepo) GetHostServiceByID(id int) (models.HostService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		SELECT hs.id, hs.host_id, hs.service_id, hs.active, hs.schedule_number, 
			hs.schedule_unit, hs.last_check, hs.status, hs.last_message, hs.created_at, hs.updated_at, 
			s.id, s.service_name, s.active, s.icon, s.created_at, s.updated_at, 
			h.host_name 
		FROM host_services AS hs 
			LEFT JOIN services AS s ON hs.service_id = s.id 
			LEFT JOIN hosts as h ON hs.host_id = h.id 
		WHERE hs.id = $1;
	`

	var hs models.HostService

	row := m.DB.QueryRowContext(ctx, query, id)

	err := row.Scan(
		&hs.ID,
		&hs.HostID,
		&hs.ServiceID,
		&hs.Active,
		&hs.ScheduleNumber,
		&hs.ScheduleUnit,
		&hs.LastCheck,
		&hs.Status,
		&hs.LastMessage,
		&hs.CreatedAt,
		&hs.UpdatedAt,
		&hs.Service.ID,
		&hs.Service.ServiceName,
		&hs.Service.Active,
		&hs.Service.Icon,
		&hs.Service.CreatedAt,
		&hs.Service.UpdatedAt,
		&hs.HostName,
	)
	if err != nil {
		log.Println(err)
		return hs, err
	}

	return hs, nil
}

func (m *postgresDBRepo) GetServicesToMonitor() ([]models.HostService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		SELECT 
			hs.id, hs.host_id, hs.service_id, hs.active, hs.schedule_number, 
			hs.schedule_unit, hs.last_check, hs.status, hs.last_message, hs.created_at, hs.updated_at, 
			s.id, s.service_name, s.active, s.icon, s.created_at, s.updated_at, 
			h.host_name 
		FROM host_services AS hs 
			LEFT JOIN services AS s ON (hs.service_id = s.id)
			LEFT JOIN hosts AS h ON (h.id = hs.host_id)
		WHERE 
			h.active = 1 AND hs.active = 1;
	`

	var services []models.HostService

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var h models.HostService
		err := rows.Scan(
			&h.ID,
			&h.HostID,
			&h.ServiceID,
			&h.Active,
			&h.ScheduleNumber,
			&h.ScheduleUnit,
			&h.LastCheck,
			&h.Status,
			&h.LastMessage,
			&h.CreatedAt,
			&h.UpdatedAt,
			&h.Service.ID,
			&h.Service.ServiceName,
			&h.Service.Active,
			&h.Service.Icon,
			&h.Service.CreatedAt,
			&h.Service.UpdatedAt,
			&h.HostName,
		)
		if err != nil {
			log.Println(err)
			return services, err
		}
		services = append(services, h)
	}

	return services, nil
}

// GetHostServiceByHostIdServiceId gets a host service by host id and service id
func (m *postgresDBRepo) GetHostServiceByHostIdServiceId(hostID, serviceID int) (models.HostService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		SELECT 
			hs.id, hs.host_id, hs.service_id, hs.active, hs.schedule_number, hs.schedule_unit, 
			hs.last_check, hs.status, hs.last_message, hs.created_at, hs.updated_at, 
			s.id, s.service_name, s.active, s.icon, s.created_at, s.updated_at, 
			h.host_name 
		FROM host_services AS hs 
			LEFT JOIN services AS s ON (hs.service_id = s.id) 
			LEFT JOIN hosts AS h ON (hs.host_id = h.id) 
		WHERE hs.host_id = $1 AND hs.service_id = $2;
	`

	var hs models.HostService

	row := m.DB.QueryRowContext(ctx, query, hostID, serviceID)

	err := row.Scan(
		&hs.ID,
		&hs.HostID,
		&hs.ServiceID,
		&hs.Active,
		&hs.ScheduleNumber,
		&hs.ScheduleUnit,
		&hs.LastCheck,
		&hs.Status,
		&hs.LastMessage,
		&hs.CreatedAt,
		&hs.UpdatedAt,
		&hs.Service.ID,
		&hs.Service.ServiceName,
		&hs.Service.Active,
		&hs.Service.Icon,
		&hs.Service.CreatedAt,
		&hs.Service.UpdatedAt,
		&hs.HostName,
	)

	if err != nil {
		log.Println("or is it this error???", err)
		return hs, err
	}

	return hs, nil
}

// InsertEvent inserts an event into the database
func (m *postgresDBRepo) InsertEvent(e models.Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stmt := `
		INSERT INTO events (
			event_type, host_service_id, host_id, service_name, host_name, message, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`

	_, err := m.DB.ExecContext(ctx, stmt,
		e.EventType,
		e.HostServiceID,
		e.HostID,
		e.ServiceName,
		e.HostName,
		e.Message,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		return err
	}

	return nil
}

// GetAllEvents gets all events
func (m *postgresDBRepo) GetAllEvents() ([]models.Event, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `
		SELECT 
			id, 
			event_type, 
			host_service_id, 
			host_id, 
			service_name, 
			host_name, 
			message, 
			created_at, 
			updated_at 
		FROM events 
		ORDER BY created_at
	`

	var events []models.Event

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ev models.Event
		err := rows.Scan(
			&ev.ID,
			&ev.EventType,
			&ev.HostServiceID,
			&ev.HostID,
			&ev.ServiceName,
			&ev.HostName,
			&ev.Message,
			&ev.CreatedAt,
			&ev.UpdatedAt,
		)
		if err != nil {
			return events, err
		}
		events = append(events, ev)
	}

	return events, nil
}

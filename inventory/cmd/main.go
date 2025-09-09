package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/google/uuid"
	inventoryv1 "github.com/sunzhqr/spacecraft/shared/pkg/proto/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const inventoryGPRCPort = 50051

type store struct {
	mu   sync.RWMutex
	data map[string]*inventoryv1.Part
}

type inventoryServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
	store *store
}

func (s *inventoryServer) GetPart(ctx context.Context, req *inventoryv1.GetPartRequest) (*inventoryv1.GetPartResponse, error) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()
	part, ok := s.store.data[req.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "part with uuid %s not found", req.GetUuid())
	}
	return &inventoryv1.GetPartResponse{
		Part: part,
	}, nil
}

func anyMatch[T comparable](needle []T, val T) bool {
	for _, v := range needle {
		if v == val {
			return true
		}
	}
	return false
}

func anyMatchStr(needle []string, val string) bool {
	for _, v := range needle {
		if v == val {
			return true
		}
	}
	return false
}

func (s *inventoryServer) ListParts(ctx context.Context, req *inventoryv1.ListPartsRequest) (*inventoryv1.ListPartsResponse, error) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()
	f := req.GetFilter()
	res := make([]*inventoryv1.Part, 0, len(s.store.data))
	for _, p := range s.store.data {
		if len(f.Uuids) > 0 && !anyMatchStr(f.Uuids, p.Uuid) {
			continue
		}
		if len(f.Names) > 0 && !anyMatchStr(f.Names, p.Name) {
			continue
		}
		if len(f.Categories) > 0 && !anyMatch(f.Categories, p.Category) {
			continue
		}
		if len(f.ManufacturerCountries) > 0 && !anyMatchStr(f.ManufacturerCountries, p.GetManufacturer().GetCountry()) {
			continue
		}
		if len(f.Tags) > 0 {
			ok := false
			for _, tag := range p.Tags {
				if anyMatchStr(f.Tags, tag) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		res = append(res, p)
	}
	return &inventoryv1.ListPartsResponse{Parts: res}, nil
}

func seed() map[string]*inventoryv1.Part {
	now := timestamppb.Now()
	id1 := uuid.New().String()
	return map[string]*inventoryv1.Part{
		id1: {
			Uuid:          id1,
			Name:          "Main Engine",
			Description:   "Core booster",
			Price:         9999.99,
			StockQuantity: 5,
			Category:      inventoryv1.Category_CATEGORY_ENGINE,
			Dimensions:    &inventoryv1.Dimensions{Length: 200, Width: 200, Height: 300, Weight: 1200},
			Manufacturer:  &inventoryv1.Manufacturer{Name: "AeroGmbH", Country: "Germany", Website: "https://aero"},
			Tags:          []string{"engine", "core"},
			Metadata:      map[string]*inventoryv1.Value{"series": {Method: &inventoryv1.Value_StringValue{StringValue: "X"}}},
			CreatedAt:     now, UpdatedAt: now,
		},
	}
}

func main() {
	st := &store{data: seed()}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", inventoryGPRCPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(grpcServer, &inventoryServer{store: st})
	reflection.Register(grpcServer)
	log.Println(fmt.Sprintf("inventory gRPC on :%d\n", inventoryGPRCPort))
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

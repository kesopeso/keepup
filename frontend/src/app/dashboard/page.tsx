'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PlusIcon, MapPinIcon } from 'lucide-react';

interface User {
    id: number;
    email: string;
    username: string;
    created_at: string;
    updated_at: string;
}

interface Trip {
    id: number;
    name: string;
    description: string;
    creator_id: number;
    status: string;
    created_at: string;
    updated_at: string;
}

export default function DashboardPage() {
    const [user, setUser] = useState<User | null>(null);
    const [trips, setTrips] = useState<Trip[]>([]);
    const [loading, setLoading] = useState(true);
    const [tripsLoading, setTripsLoading] = useState(true);
    const router = useRouter();

    useEffect(() => {
        const token = localStorage.getItem('access_token');
        const storedUser = localStorage.getItem('user');

        if (!token) {
            router.push('/auth/login');
            return;
        }

        if (storedUser) {
            setUser(JSON.parse(storedUser));
            setLoading(false);
        }

        // Fetch user's trips
        fetchTrips();
    }, [router]);

    const fetchTrips = async () => {
        const token = localStorage.getItem('access_token');
        if (!token) return;

        try {
            const response = await fetch('http://localhost:8080/api/v1/trips', {
                headers: {
                    'Authorization': `Bearer ${token}`,
                },
            });

            if (response.ok) {
                const data = await response.json();
                setTrips(data.trips || []);
            }
        } catch (error) {
            console.error('Failed to fetch trips:', error);
        } finally {
            setTripsLoading(false);
        }
    };

    const handleLogout = () => {
        localStorage.removeItem('access_token');
        localStorage.removeItem('refresh_token');
        localStorage.removeItem('user');
        router.push('/auth/login');
    };

    if (loading) {
        return (
            <div className="min-h-screen flex items-center justify-center">
                <p>Loading...</p>
            </div>
        );
    }

    if (!user) {
        return null;
    }

    return (
        <div className="min-h-screen bg-gray-50">
            <div className="container mx-auto py-8 px-4">
                <div className="flex justify-between items-center mb-8">
                    <div>
                        <h1 className="text-3xl font-bold">Dashboard</h1>
                        <p className="text-muted-foreground">
                            Welcome back, <span className="font-semibold">{user.username}</span>!
                        </p>
                    </div>
                    <Button variant="outline" onClick={handleLogout}>
                        Logout
                    </Button>
                </div>
                
                <div className="space-y-6">
                    <div className="flex justify-between items-center">
                        <h2 className="text-2xl font-semibold">My Trips</h2>
                        <Button asChild>
                            <Link href="/trips/create">
                                <PlusIcon className="w-4 h-4 mr-2" />
                                Create Trip
                            </Link>
                        </Button>
                    </div>

                    {tripsLoading ? (
                        <Card>
                            <CardContent className="py-8">
                                <p className="text-center text-muted-foreground">Loading trips...</p>
                            </CardContent>
                        </Card>
                    ) : trips.length === 0 ? (
                        <Card>
                            <CardContent className="py-12">
                                <div className="text-center space-y-4">
                                    <MapPinIcon className="w-12 h-12 mx-auto text-muted-foreground" />
                                    <div>
                                        <h3 className="text-lg font-semibold mb-2">No trips yet</h3>
                                        <p className="text-muted-foreground mb-4">
                                            Create your first trip to start tracking your adventures!
                                        </p>
                                        <Button asChild>
                                            <Link href="/trips/create">
                                                <PlusIcon className="w-4 h-4 mr-2" />
                                                Create Your First Trip
                                            </Link>
                                        </Button>
                                    </div>
                                </div>
                            </CardContent>
                        </Card>
                    ) : (
                        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                            {trips.map((trip) => (
                                <Card key={trip.id} className="hover:shadow-md transition-shadow">
                                    <CardHeader>
                                        <div className="flex justify-between items-start mb-2">
                                            <CardTitle className="text-lg">{trip.name}</CardTitle>
                                            <span className={`px-2 py-1 rounded-full text-xs font-medium ${
                                                trip.status === 'active' 
                                                    ? 'bg-green-100 text-green-800'
                                                    : trip.status === 'created'
                                                    ? 'bg-blue-100 text-blue-800'
                                                    : 'bg-gray-100 text-gray-800'
                                            }`}>
                                                {trip.status}
                                            </span>
                                        </div>
                                        {trip.description && (
                                            <CardDescription>{trip.description}</CardDescription>
                                        )}
                                    </CardHeader>
                                    <CardContent>
                                        <div className="flex justify-between items-center">
                                            <span className="text-sm text-muted-foreground">
                                                Created {new Date(trip.created_at).toLocaleDateString()}
                                            </span>
                                            <Button variant="outline" size="sm" asChild>
                                                <Link href={`/trips/${trip.id}`}>
                                                    View Trip
                                                </Link>
                                            </Button>
                                        </div>
                                    </CardContent>
                                </Card>
                            ))}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
'use client';

import { useEffect, useState } from 'react';
import { useRouter, useParams } from 'next/navigation';
import { Button } from '@/components/ui/button';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { MenuIcon } from 'lucide-react';
import GoogleMap from '@/components/GoogleMap';

interface Trip {
    id: number;
    name: string;
    description: string;
    creator_id: number;
    status: string;
    created_at: string;
    updated_at: string;
}


export default function TripPage() {
    const [trip, setTrip] = useState<Trip | null>(null);
    const [loading, setLoading] = useState(true);
    const [menuOpen, setMenuOpen] = useState(false);
    const [endingTrip, setEndingTrip] = useState(false);
    const [startingTrip, setStartingTrip] = useState(false);
    const router = useRouter();
    const params = useParams();
    const tripId = params.id as string;

    useEffect(() => {
        const token = localStorage.getItem('access_token');
        if (!token) {
            router.push('/auth/login');
            return;
        }

        fetchTrip();
    }, [tripId, router]);

    const fetchTrip = async () => {
        const token = localStorage.getItem('access_token');
        if (!token) return;

        try {
            const response = await fetch(
                `http://localhost:8080/api/v1/trips/${tripId}`,
                {
                    headers: {
                        Authorization: `Bearer ${token}`,
                    },
                }
            );

            if (response.ok) {
                const data = await response.json();
                setTrip(data.trip);
            } else if (response.status === 404) {
                router.push('/dashboard');
            }
        } catch (error) {
            console.error('Failed to fetch trip:', error);
            router.push('/dashboard');
        } finally {
            setLoading(false);
        }
    };

    const handleEndTrip = async () => {
        if (!trip || endingTrip) return;

        const confirmed = window.confirm(
            'Are you sure you want to end this trip? This action cannot be undone.'
        );
        if (!confirmed) return;

        setEndingTrip(true);
        const token = localStorage.getItem('access_token');

        try {
            const response = await fetch(
                `http://localhost:8080/api/v1/trips/${trip.id}/end`,
                {
                    method: 'PUT',
                    headers: {
                        Authorization: `Bearer ${token}`,
                    },
                }
            );

            if (response.ok) {
                // Refresh trip data to show updated status
                await fetchTrip();
                setMenuOpen(false);
            } else {
                alert('Failed to end trip. Please try again.');
            }
        } catch (error) {
            console.error('Failed to end trip:', error);
            alert('Failed to end trip. Please try again.');
        } finally {
            setEndingTrip(false);
        }
    };

    const handleStartTrip = async () => {
        if (!trip || startingTrip) return;

        setStartingTrip(true);
        const token = localStorage.getItem('access_token');

        try {
            const response = await fetch(
                `http://localhost:8080/api/v1/trips/${trip.id}/start`,
                {
                    method: 'PUT',
                    headers: {
                        Authorization: `Bearer ${token}`,
                    },
                }
            );

            if (response.ok) {
                // Refresh trip data to show updated status
                await fetchTrip();
            } else {
                const data = await response.json();
                alert(data.error || 'Failed to start trip. Please try again.');
            }
        } catch (error) {
            console.error('Failed to start trip:', error);
            alert('Failed to start trip. Please try again.');
        } finally {
            setStartingTrip(false);
        }
    };

    if (loading) {
        return (
            <div className="h-screen flex items-center justify-center">
                <p>Loading trip...</p>
            </div>
        );
    }

    if (!trip) {
        return (
            <div className="h-screen flex items-center justify-center">
                <p>Trip not found</p>
            </div>
        );
    }

    return (
        <div className="h-screen flex flex-col relative">
            {/* Header with trip name and hamburger menu */}
            <div className="absolute top-4 left-4 right-4 z-10 flex justify-between items-center">
                <div className="bg-white/90 backdrop-blur-sm rounded-lg px-4 py-2 shadow-sm">
                    <h1 className="font-semibold text-lg">{trip.name}</h1>
                    <p className="text-sm text-gray-600">
                        Status:{' '}
                        <span
                            className={`font-medium ${
                                trip.status === 'active'
                                    ? 'text-green-600'
                                    : trip.status === 'created'
                                      ? 'text-blue-600'
                                      : 'text-gray-600'
                            }`}
                        >
                            {trip.status}
                        </span>
                    </p>
                </div>

                <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
                    <DropdownMenuTrigger
                        className="inline-flex items-center justify-center rounded-md border border-input bg-white/90 backdrop-blur-sm shadow-sm h-9 w-9 hover:bg-accent hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                        onClick={() => setMenuOpen(!menuOpen)}
                    >
                        <MenuIcon className="h-5 w-5" />
                    </DropdownMenuTrigger>
                    <DropdownMenuContent open={menuOpen}>
                        <DropdownMenuItem
                            onClick={() => router.push('/dashboard')}
                        >
                            Back to Dashboard
                        </DropdownMenuItem>
                        {trip.status === 'active' && (
                            <DropdownMenuItem
                                destructive
                                onClick={handleEndTrip}
                                className="cursor-pointer"
                            >
                                {endingTrip ? 'Ending Trip...' : 'End Trip'}
                            </DropdownMenuItem>
                        )}
                    </DropdownMenuContent>
                </DropdownMenu>
            </div>

            {/* Google Maps - Full screen */}
            <div className="flex-1 relative" style={{ minHeight: '400px' }}>
                <GoogleMap className="w-full h-full absolute inset-0" />

                {/* Start Trip CTA - Only show for 'created' status */}
                {trip.status === 'created' && (
                    <div className="absolute inset-0 flex items-center justify-center z-20">
                        <div className="bg-white/95 backdrop-blur-sm rounded-xl shadow-lg p-8 max-w-sm mx-4 text-center">
                            <div className="mb-6">
                                <div className="w-16 h-16 bg-blue-100 rounded-full flex items-center justify-center mx-auto mb-4">
                                    <svg
                                        className="w-8 h-8 text-blue-600"
                                        fill="none"
                                        stroke="currentColor"
                                        viewBox="0 0 24 24"
                                    >
                                        <path
                                            strokeLinecap="round"
                                            strokeLinejoin="round"
                                            strokeWidth={2}
                                            d="M14.828 14.828a4 4 0 01-5.656 0M9 10h1m4 0h1m-6.5-4h3c.9 0 1.75.09 2.5.26M5.5 6A7.5 7.5 0 0013 14.5M5.5 6v6a7.5 7.5 0 007.5 7.5h0a7.5 7.5 0 007.5-7.5v-6M5.5 6H3"
                                        />
                                    </svg>
                                </div>
                                <h2 className="text-xl font-semibold text-gray-900 mb-2">
                                    Ready to Start Your Trip?
                                </h2>
                                <p className="text-gray-600 text-sm">
                                    Click the button below to start tracking
                                    your location and begin your adventure!
                                </p>
                            </div>
                            <Button
                                onClick={handleStartTrip}
                                disabled={startingTrip}
                                size="lg"
                                className="w-full"
                            >
                                {startingTrip ? (
                                    <>
                                        <svg
                                            className="animate-spin -ml-1 mr-3 h-5 w-5 text-white"
                                            xmlns="http://www.w3.org/2000/svg"
                                            fill="none"
                                            viewBox="0 0 24 24"
                                        >
                                            <circle
                                                className="opacity-25"
                                                cx="12"
                                                cy="12"
                                                r="10"
                                                stroke="currentColor"
                                                strokeWidth="4"
                                            ></circle>
                                            <path
                                                className="opacity-75"
                                                fill="currentColor"
                                                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                                            ></path>
                                        </svg>
                                        Starting Trip...
                                    </>
                                ) : (
                                    'Start Trip'
                                )}
                            </Button>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}


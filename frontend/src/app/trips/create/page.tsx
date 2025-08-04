'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { ArrowLeftIcon } from 'lucide-react';

export default function CreateTripPage() {
    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [password, setPassword] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const router = useRouter();

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        setError('');

        try {
            const response = await fetch('http://localhost:8080/api/v1/trips', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                credentials: 'include', // Include cookies
                body: JSON.stringify({ name, description, password }),
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || 'Failed to create trip');
            }

            // Redirect to the newly created trip
            router.push(`/trips/${data.trip.id}`);
        } catch (err) {
            setError(err instanceof Error ? err.message : 'An error occurred');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen bg-gray-50">
            <div className="container mx-auto py-8 px-4">
                <div className="max-w-2xl mx-auto">
                    <div className="mb-6">
                        <Button variant="outline" asChild className="mb-4">
                            <Link href="/dashboard">
                                <ArrowLeftIcon className="w-4 h-4 mr-2" />
                                Back to Dashboard
                            </Link>
                        </Button>
                        <h1 className="text-3xl font-bold">Create New Trip</h1>
                        <p className="text-muted-foreground">
                            Set up your trip and invite others to join with the trip password.
                        </p>
                    </div>

                    <Card>
                        <CardHeader>
                            <CardTitle>Trip Details</CardTitle>
                            <CardDescription>
                                Provide basic information about your trip
                            </CardDescription>
                        </CardHeader>
                        <form onSubmit={handleSubmit}>
                            <CardContent className="space-y-4">
                                {error && (
                                    <div className="text-red-600 text-sm bg-red-50 p-3 rounded-md">
                                        {error}
                                    </div>
                                )}
                                <div className="space-y-2">
                                    <Label htmlFor="name">Trip Name *</Label>
                                    <Input
                                        id="name"
                                        type="text"
                                        placeholder="Weekend Camping Trip"
                                        value={name}
                                        onChange={(e) => setName(e.target.value)}
                                        required
                                        maxLength={100}
                                        disabled={loading}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="description">Description (Optional)</Label>
                                    <textarea
                                        id="description"
                                        className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                                        placeholder="A fun weekend camping trip in the mountains..."
                                        value={description}
                                        onChange={(e) => setDescription(e.target.value)}
                                        maxLength={500}
                                        disabled={loading}
                                        rows={3}
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        {description.length}/500 characters
                                    </p>
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="password">Trip Password *</Label>
                                    <Input
                                        id="password"
                                        type="text"
                                        placeholder="Enter a password for others to join"
                                        value={password}
                                        onChange={(e) => setPassword(e.target.value)}
                                        required
                                        minLength={4}
                                        maxLength={50}
                                        disabled={loading}
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        Share this password with friends so they can join your trip
                                    </p>
                                </div>
                            </CardContent>
                            <CardFooter className="flex gap-4">
                                <Button
                                    type="button"
                                    variant="outline"
                                    onClick={() => router.back()}
                                    disabled={loading}
                                >
                                    Cancel
                                </Button>
                                <Button type="submit" disabled={loading}>
                                    {loading ? 'Creating trip...' : 'Create Trip'}
                                </Button>
                            </CardFooter>
                        </form>
                    </Card>
                </div>
            </div>
        </div>
    );
}
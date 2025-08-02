'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

interface User {
    id: number;
    email: string;
    username: string;
    created_at: string;
    updated_at: string;
}

export default function DashboardPage() {
    const [user, setUser] = useState<User | null>(null);
    const [loading, setLoading] = useState(true);
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
    }, [router]);

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
                    <h1 className="text-3xl font-bold">Dashboard</h1>
                    <Button variant="outline" onClick={handleLogout}>
                        Logout
                    </Button>
                </div>
                
                <Card className="max-w-2xl">
                    <CardHeader>
                        <CardTitle>Welcome!</CardTitle>
                    </CardHeader>
                    <CardContent>
                        <div className="space-y-4">
                            <p className="text-2xl">
                                Hello <span className="font-semibold text-primary">{user.username}</span>!
                            </p>
                            <div className="space-y-2 text-sm text-muted-foreground">
                                <p><strong>Email:</strong> {user.email}</p>
                                <p><strong>Member since:</strong> {new Date(user.created_at).toLocaleDateString()}</p>
                            </div>
                        </div>
                    </CardContent>
                </Card>
            </div>
        </div>
    );
}